package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"log"
	"math/big"
	"os"
	"strings"
	"time"
)

//const rawABI = `
//[
//    {
//      "anonymous": false,
//      "inputs": [
//        {
//          "indexed": true,
//          "internalType": "address",
//          "name": "from",
//          "type": "address"
//        },
//        {
//          "indexed": false,
//          "internalType": "uint256",
//          "name": "amount",
//          "type": "uint256"
//        }
//      ],
//      "name": "FallbackEvent",
//      "type": "event"
//    },
//    {
//      "anonymous": false,
//      "inputs": [
//        {
//          "indexed": true,
//          "internalType": "address",
//          "name": "previousOwner",
//          "type": "address"
//        },
//        {
//          "indexed": true,
//          "internalType": "address",
//          "name": "newOwner",
//          "type": "address"
//        }
//      ],
//      "name": "OwnershipTransferred",
//      "type": "event"
//    },
//    {
//      "anonymous": false,
//      "inputs": [
//        {
//          "indexed": true,
//          "internalType": "address",
//          "name": "from",
//          "type": "address"
//        },
//        {
//          "indexed": false,
//          "internalType": "uint256",
//          "name": "amount",
//          "type": "uint256"
//        }
//      ],
//      "name": "ReceiveEvent",
//      "type": "event"
//    },
//    {
//      "anonymous": false,
//      "inputs": [
//        {
//          "indexed": true,
//          "internalType": "address",
//          "name": "receiver",
//          "type": "address"
//        },
//        {
//          "indexed": false,
//          "internalType": "uint256",
//          "name": "amount",
//          "type": "uint256"
//        }
//      ],
//      "name": "WithDrawEvent",
//      "type": "event"
//    },
//    {
//      "stateMutability": "payable",
//      "type": "fallback"
//    },
//    {
//      "inputs": [],
//      "name": "destory",
//      "outputs": [],
//      "stateMutability": "nonpayable",
//      "type": "function"
//    },
//    {
//      "inputs": [],
//      "name": "getBalance",
//      "outputs": [
//        {
//          "internalType": "uint256",
//          "name": "",
//          "type": "uint256"
//        }
//      ],
//      "stateMutability": "view",
//      "type": "function"
//    },
//    {
//      "inputs": [],
//      "name": "owner",
//      "outputs": [
//        {
//          "internalType": "address",
//          "name": "",
//          "type": "address"
//        }
//      ],
//      "stateMutability": "view",
//      "type": "function"
//    },
//    {
//      "inputs": [],
//      "name": "renounceOwnership",
//      "outputs": [],
//      "stateMutability": "nonpayable",
//      "type": "function"
//    },
//    {
//      "inputs": [
//        {
//          "internalType": "address",
//          "name": "newOwner",
//          "type": "address"
//        }
//      ],
//      "name": "transferOwnership",
//      "outputs": [],
//      "stateMutability": "nonpayable",
//      "type": "function"
//    },
//    {
//      "inputs": [
//        {
//          "internalType": "address payable",
//          "name": "receiver",
//          "type": "address"
//        },
//        {
//          "internalType": "uint256",
//          "name": "amount",
//          "type": "uint256"
//        }
//      ],
//      "name": "withDraw",
//      "outputs": [],
//      "stateMutability": "nonpayable",
//      "type": "function"
//    },
//    {
//      "stateMutability": "payable",
//      "type": "receive"
//    }
//  ]
//`

type (
	RawABIResponse struct {
		Status  *string `json:"status"`
		Message *string `json:"message"`
		Result  *string `json:"result"`
	}

	EventLog struct {
		Name    string
		Message string
		Tx      string
	}
)

func (e EventLog) String() string {
	return fmt.Sprintf("EventLog<Name=%s, Message=%s, Tx=%s>", e.Name, e.Message, e.Tx)
}
func (e EventLog) TxUrl() string {
	return fmt.Sprintf("https://ropsten.etherscan.io/tx/%s", e.Tx)
}

//func EtherToWei(val *big.Int) *big.Int {
//	return new(big.Int).Mul(val, big.NewInt(params.Ether))
//}

func WeiToEther(val *big.Int) *big.Float {
	//return new(big.Int).Div(val, big.NewInt(params.Ether))
	return new(big.Float).Quo(new(big.Float).SetInt(val), big.NewFloat(params.Ether))
}

func HashToShortAddress(s common.Hash) string {
	address := common.HexToAddress(s.Hex()).String()
	return ShortAddress(address)
}

func ShortAddress(address string) string {
	return strings.Join([]string{address[:6], address[len(address)-4:]}, "...")
}

func GetContractRawABI(address string, apiKey string) (*RawABIResponse, error) {
	client := resty.New()
	rawABIResponse := &RawABIResponse{}
	resp, err := client.R().
		SetQueryParams(map[string]string{
			"module":  "contract",
			"action":  "getabi",
			"address": address,
			"apikey":  apiKey,
		}).
		SetResult(rawABIResponse).
		Get("https://api-ropsten.etherscan.io/api")

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(fmt.Sprintf("Get contract raw abi failed: %s", resp))
	}
	if *rawABIResponse.Status != "1" {
		return nil, fmt.Errorf(fmt.Sprintf("Get contract raw abi failed: %s", *rawABIResponse.Result))
	}

	return rawABIResponse, nil
}

func ParseLog(vLog types.Log, contractABI abi.ABI) (*EventLog, error) {
	event, err := contractABI.EventByID(vLog.Topics[0])
	if err != nil {
		return nil, err
	}
	log.Printf("Recevied %s", event.Name)
	log.Printf("TxHash: %s\n", vLog.TxHash)
	dataMap := make(map[string]interface{})
	if len(vLog.Data) != 0 {
		err = contractABI.UnpackIntoMap(dataMap, "WithDrawEvent", vLog.Data)
		if err != nil {
			return nil, err
		}
	}

	eventLog := new(EventLog)
	eventLog.Tx = vLog.TxHash.Hex()
	eventLog.Name = event.Name
	switch event.Name {
	case "FallbackEvent", "ReceiveEvent":
		fromAddress := HashToShortAddress(vLog.Topics[1])
		amount := dataMap["amount"].(*big.Int)
		eventLog.Message = fmt.Sprintf("Received %s ETH from %s", WeiToEther(amount).String(), fromAddress)
	case "OwnershipTransferred":
		previousOwnerAddress := HashToShortAddress(vLog.Topics[1])
		newOwnerAddress := HashToShortAddress(vLog.Topics[2])
		eventLog.Message = fmt.Sprintf("OwnershipTransferred from %s to %s", previousOwnerAddress, newOwnerAddress)
	case "WithDrawEvent":
		receiverAddress := HashToShortAddress(vLog.Topics[1])
		amount := dataMap["amount"].(*big.Int)
		eventLog.Message = fmt.Sprintf("%s withdraw %s ETH", receiverAddress, WeiToEther(amount).String())
	default:
		eventLog.Message = "Unhandled Event"
	}
	return eventLog, nil
}

//func QueryLogs(ropstenHttpsEndPoint string, ropstenFaucetContractAddress string, blockNumber int64) ([]types.Log, error) {
//	client, err := ethclient.Dial(ropstenHttpsEndPoint)
//	if err != nil {
//		return nil, err
//	}
//
//	contractAddress := common.HexToAddress(ropstenFaucetContractAddress)
//	blockLimit := big.NewInt(blockNumber)
//	query := ethereum.FilterQuery{
//		FromBlock: blockLimit,
//		ToBlock:   blockLimit,
//		Addresses: []common.Address{contractAddress},
//	}
//	// query contract event in block 12218203
//	logs, err := client.FilterLogs(context.Background(), query)
//	if err != nil {
//		return nil, err
//	}
//	return logs, nil
//}

func SubscribeLogs(ropstenWebsocketEndPoint, ropstenFaucetContractAddress string) (ethereum.Subscription, chan types.Log, error) {
	client, err := ethclient.Dial(ropstenWebsocketEndPoint)
	if err != nil {
		return nil, nil, err
	}
	contractAddress := common.HexToAddress(ropstenFaucetContractAddress)
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	log.Println("Connecting websocket server...")
	if err != nil {
		return nil, nil, err
	}
	return sub, logs, err
}

func getAccountBalance(ropstenHttpsEndPoint string, accountAddress string) string {
	client, err := ethclient.Dial(ropstenHttpsEndPoint)
	if err != nil {
		log.Printf("Failed to get balance of %s, err: %s", accountAddress, err)
		return "unknown"
	}
	account := common.HexToAddress(accountAddress)
	balance, err := client.BalanceAt(context.Background(), account, nil)
	if err != nil {
		log.Printf("Failed to get balance of %s, err: %s", accountAddress, err)
		return "unknown"
	}
	return WeiToEther(balance).String()
}

// PostEventToSlack
// 消息构造参考SDK中的example写法以及源码中的测试用例代码来写
// 配合工具网站来调试 https://app.slack.com/block-kit-builder/T020VUBR99V
func PostEventToSlack(webhookUrl string, eventLog *EventLog, faucetBalance, minerBalance string) error {
	headerText := slack.NewTextBlockObject("plain_text", fmt.Sprintf("📣%s", eventLog.Name), true, false)
	headerSection := slack.NewHeaderBlock(headerText)

	viewTxButton := slack.NewButtonBlockElement("viewTx", eventLog.Tx,
		slack.NewTextBlockObject(
			"plain_text", "View Detail", false, false),
	)
	viewTxButton.URL = eventLog.TxUrl()

	bodyText := slack.NewTextBlockObject("mrkdwn", eventLog.Message, false, false)
	bodySection := slack.NewSectionBlock(bodyText, nil, slack.NewAccessory(viewTxButton))
	balanceText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Faucet Balance*: %s\n*Miner Balance*: %s\n", faucetBalance, minerBalance), false, false)
	balanceSection := slack.NewSectionBlock(balanceText, nil, nil)
	msg := slack.WebhookMessage{
		Blocks: &slack.Blocks{
			BlockSet: []slack.Block{
				headerSection,
				bodySection,
				balanceSection,
				slack.NewDividerBlock(),
			}},
	}

	// 仅用于调试时查看生成的消息json数据
	//b, err := json.MarshalIndent(msg, "", "    ")
	//if err != nil {
	//	return err
	//}
	//fmt.Println(string(b))

	return slack.PostWebhook(webhookUrl, &msg)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: loading .env file failed")
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	faucetChannelWebhookUrl := os.Getenv("FAUCET_CHANNEL_WEBHOOK_URL")
	etherscanAPIKey := os.Getenv("ETHERSCAN_API_KEY")

	ropstenHttpsEndpoint := os.Getenv("ROPSTEN_HTTPS_ENDPOINT")
	ropstenWebsocketEndpoint := os.Getenv("ROPSTEN_WEBSOCKET_ENDPOINT")
	ropstenFaucetContractAddress := os.Getenv("ROPSTEN_FAUCET_CONTRACT_ADDRESS")
	minerAddress := os.Getenv("MINER_ADDRESS")

	//const ropstenHttpsEndpoint = "http://127.0.0.1:8545/"
	//const ropstenWebsocketEndpoint = "ws://127.0.0.1:8545/"
	//const ropstenFaucetContractAddress = "0x5FbDB2315678afecb367f032d93F642f64180aa3"
	//const minerAddress = "0x206AaB6b3e64e812479E287715fe40b2d7BDE67d"

	//const blockNumber = 12218757

	rawABIResponse, err := GetContractRawABI(ropstenFaucetContractAddress, etherscanAPIKey)
	if err != nil {
		log.Fatal(err)
	}

	contractABI, err := abi.JSON(strings.NewReader(*rawABIResponse.Result))
	//contractABI, err := abi.JSON(strings.NewReader(rawABI))
	if err != nil {
		log.Fatal(err)
	}

SUB:
	sub, logs, err := SubscribeLogs(ropstenWebsocketEndpoint, ropstenFaucetContractAddress)
	if err != nil {
		log.Printf("Subscribe logs error: %s\n", err)
		log.Printf("rery in 3 seconds...")
		time.Sleep(time.Second * 3)
		goto SUB
	}
	log.Printf("Waiting for event..")
	for {
		select {
		case err := <-sub.Err():
			// todo: Websocket端口后如何自动重连
			log.Printf("Connect error: %s,  Reconncting...\n", err)
			goto SUB
		case vLog := <-logs:
			if eventLog, err := ParseLog(vLog, contractABI); err != nil {
				log.Printf("Parse Log failed, err: %s", err)
			} else {
				faucetBalance := getAccountBalance(ropstenHttpsEndpoint, ropstenFaucetContractAddress)
				minerBalance := getAccountBalance(ropstenHttpsEndpoint, minerAddress)
				err := PostEventToSlack(faucetChannelWebhookUrl, eventLog, faucetBalance, minerBalance)
				if err != nil {
					log.Printf("Post message to Slack Failed, EventLog: %s, err: %s", eventLog, err)
				}
			}
		}
	}

	// 直接查询并解析日志
	//logs, err := QueryLogs(ropstenHttpsEndpoint, ropstenFaucetContractAddress, blockNumber)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//faucetBalance := getAccountBalance(ropstenHttpsEndpoint, ropstenFaucetContractAddress)
	//minerBalance := getAccountBalance(ropstenHttpsEndpoint, minerAddress)
	//// parse logs
	//for _, vLog := range logs {
	//	if eventLog, err := ParseLog(vLog, contractABI); err != nil {
	//		log.Printf("Parse Log failed, err: %s", err)
	//	} else {
	//		err := PostEventToSlack(faucetChannelWebhookUrl, eventLog, faucetBalance, minerBalance)
	//		if err != nil {
	//			fmt.Println(err.Error())
	//			log.Printf("Post message to Slack Failed, EventLog: %s, err: %s", eventLog, err)
	//		}
	//	}
	//}
}
