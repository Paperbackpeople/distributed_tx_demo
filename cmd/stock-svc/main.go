// cmd/stock-svc/main.go
package main

import (
	"fmt"
	"net"
	"os"

	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/cmd/stock-svc/server"

	"google.golang.org/grpc"
)

func main() {
	port := env("PORT", "6002")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	txv1.RegisterStockSvcServer(s, &server.Stock{})
	fmt.Println("stock-svc listen", port)
	if err := s.Serve(lis); err != nil {
		panic(err)
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
