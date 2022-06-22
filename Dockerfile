FROM golang:1.18.1-alpine as builder

ENV GO111MODULE=on \
    CGO_ENABLED=0

WORKDIR /build

# no such package in aarch64
# RUN apk add --no-cache upx

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -o facucet-lisenter
# RUN upx facucet-lisenter

#FROM alpine:3
FROM scratch
WORKDIR /app
# 修复使用scratch时报没有证书的错误
# copy the ca-certificate.crt from the build stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /build/facucet-lisenter .

CMD ["/app/facucet-lisenter"]
