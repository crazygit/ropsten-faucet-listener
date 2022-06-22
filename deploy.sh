rsync -avz --delete . --exclude=.idea/ oracle-dream:~/src/FaucetEventListener
ssh oracle-dream "cd ~/src/FaucetEventListener && docker-compose up -d --build"
