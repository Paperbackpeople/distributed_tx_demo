package main

import (
	"fmt"
	"net"
	"os"

	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/cmd/pay-svc/server"

	"google.golang.org/grpc"
)

func main() {
	port := env("PORT", "6003")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	txv1.RegisterPaySvcServer(s, &server.Pay{})
	fmt.Println("pay-svc listen", port)
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
