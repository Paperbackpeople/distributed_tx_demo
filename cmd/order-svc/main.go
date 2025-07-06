// cmd/order-svc/main.go
package main

import (
	"fmt"
	"net"
	"os"

	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/cmd/order-svc/server"

	"google.golang.org/grpc"
)

func main() {
	port := env("PORT", "6001")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	txv1.RegisterOrderSvcServer(s, &server.Order{})
	fmt.Println("order-svc listen", port)
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
