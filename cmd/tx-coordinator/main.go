package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"log"
	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
	"google.golang.org/grpc"
)

type participant struct {
	try     func(context.Context) (*txv1.Ack, error)
	confirm func(context.Context) (*txv1.Ack, error)
	cancel  func(context.Context) (*txv1.Ack, error)
}

func main() {
	http.HandleFunc("/placeOrder", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		gid := infra.NewGID()

		// 创建 gRPC 连接（端口记得和实际一致）
		connOrder, _ := grpc.Dial("order-svc:6001", grpc.WithInsecure())
		connStock, _ := grpc.Dial("stock-svc:6002", grpc.WithInsecure())
		connPay, _ := grpc.Dial("pay-svc:6003", grpc.WithInsecure())
		defer connOrder.Close()
		defer connStock.Close()
		defer connPay.Close()

		orderCli := txv1.NewOrderSvcClient(connOrder)
		stockCli := txv1.NewStockSvcClient(connStock)
		payCli := txv1.NewPaySvcClient(connPay)

		ps := []participant{
			{
				try: func(c context.Context) (*txv1.Ack, error) {
					return orderCli.Try(c, &txv1.OrderTry{
						Gid:      gid,
						UserId:   1,
						TotalAmt: 100,
						Items:    []*txv1.OrderItem{{ProductId: 1, Qty: 1, Price: 100}},
					})
				},
				confirm: func(c context.Context) (*txv1.Ack, error) {
					return orderCli.Confirm(c, &txv1.Gid{Gid: gid})
				},
				cancel: func(c context.Context) (*txv1.Ack, error) {
					return orderCli.Cancel(c, &txv1.Gid{Gid: gid})
				},
			},
			{
				try: func(c context.Context) (*txv1.Ack, error) {
					return stockCli.Try(c, &txv1.StockTry{Gid: gid, ProductId: 1, Qty: 1})
				},
				confirm: func(c context.Context) (*txv1.Ack, error) {
					return stockCli.Confirm(c, &txv1.Gid{Gid: gid})
				},
				cancel: func(c context.Context) (*txv1.Ack, error) {
					return stockCli.Cancel(c, &txv1.Gid{Gid: gid})
				},
			},
			{
				try: func(c context.Context) (*txv1.Ack, error) {
					return payCli.Try(c, &txv1.PayTry{Gid: gid, UserId: 1, Amount: 100})
				},
				confirm: func(c context.Context) (*txv1.Ack, error) {
					return payCli.Confirm(c, &txv1.Gid{Gid: gid})
				},
				cancel: func(c context.Context) (*txv1.Ack, error) {
					return payCli.Cancel(c, &txv1.Gid{Gid: gid})
				},
			},
		}

		// phase 1: Try
		allOk := true
		for i, p := range ps {
			ack, err := p.try(ctx)
			log.Printf("TRY-%d result: ack=%+v err=%v", i, ack, err)
			if err != nil || !ack.Ok {
				allOk = false
				break
			}
		}

		// phase 2: Confirm / Cancel
		if allOk {
			for _, p := range ps {
				p.confirm(ctx)
			}
			fmt.Fprint(w, "OK "+gid)
		} else {
			for _, p := range ps {
				p.cancel(ctx)
			}
			http.Error(w, "ROLLBACK "+gid, 500)
		}
	})

	fmt.Println("coordinator listen :7000")
	http.ListenAndServe(":7000", nil)
}
