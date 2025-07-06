# -------- 第一阶段：编译 Go 服务 --------
FROM golang:1.24-alpine as builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 使用 build-arg 控制要 build 的服务名（order-svc/stock-svc/pay-svc/tx-coordinator）
ARG SERVICE
RUN cd cmd/${SERVICE} && go build -o /main

# -------- 第二阶段：极简运行环境 --------
FROM alpine:3.18
WORKDIR /app

COPY --from=builder /main /main

EXPOSE 6001 6002 6003 7000

ENTRYPOINT ["/main"]