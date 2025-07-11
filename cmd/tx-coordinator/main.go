package main

import (
	"context"
	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
	"fmt"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type sagaStep struct {
	execute    func(context.Context) (*txv1.Ack, error)
	compensate func(context.Context) (*txv1.Ack, error)
}

func main() {
	http.HandleFunc("/placeOrder", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		gid := infra.NewGID()

		connOrder, _ := grpc.NewClient("order-svc:6001", grpc.WithTransportCredentials(insecure.NewCredentials()))
		connStock, _ := grpc.NewClient("stock-svc:6002", grpc.WithTransportCredentials(insecure.NewCredentials()))
		connPay, _ := grpc.NewClient("pay-svc:6003", grpc.WithTransportCredentials(insecure.NewCredentials()))
		defer connOrder.Close()
		defer connStock.Close()
		defer connPay.Close()

		orderCli := txv1.NewOrderSvcClient(connOrder)
		stockCli := txv1.NewStockSvcClient(connStock)
		payCli := txv1.NewPaySvcClient(connPay)

		steps := []sagaStep{
			{
				execute: func(c context.Context) (*txv1.Ack, error) {
					return orderCli.Execute(c, &txv1.OrderSagaRequest{
						Gid:      gid,
						UserId:   1,
						TotalAmt: 100,
						Items:    []*txv1.OrderItem{{ProductId: 1, Qty: 1, Price: 100}},
					})
				},
				compensate: func(c context.Context) (*txv1.Ack, error) {
					return orderCli.Compensate(c, &txv1.OrderSagaRequest{Gid: gid})
				},
			},
			{
				execute: func(c context.Context) (*txv1.Ack, error) {
					return stockCli.Execute(c, &txv1.StockSagaRequest{
						Gid: gid, ProductId: 1, Qty: 1,
					})
				},
				compensate: func(c context.Context) (*txv1.Ack, error) {
					return stockCli.Compensate(c, &txv1.StockSagaRequest{
						Gid: gid, ProductId: 1, Qty: 1,
					})
				},
			},
			{
				execute: func(c context.Context) (*txv1.Ack, error) {
					return payCli.Execute(c, &txv1.PaySagaRequest{
						Gid: gid, UserId: 1, TotalAmt: 100,
					})
				},
				compensate: func(c context.Context) (*txv1.Ack, error) {
					return payCli.Compensate(c, &txv1.PaySagaRequest{
						Gid: gid, UserId: 1, TotalAmt: 100,
					})
				},
			},
		}

		var executed int
		var failAck *txv1.Ack
		var failErr error
		// 1. 顺序执行 execute
		for i, step := range steps {
			ack, err := step.execute(ctx)
			log.Printf("SAGA EXECUTE-%d result: ack=%+v err=%v", i, ack, err)
			if err != nil || !ack.Ok {
				executed = i // 只回滚已成功的前 N 步
				failAck = ack
				failErr = err
				break
			}
			executed = i + 1
		}

		// 2. 如果有失败，逆序补偿
		if executed < len(steps) {
			for j := executed - 1; j >= 0; j-- {
				ack, err := steps[j].compensate(ctx)
				log.Printf("SAGA COMPENSATE-%d result: ack=%+v err=%v", j, ack, err)
			}
			msg := fmt.Sprintf("ROLLBACK %s fail-ack=%+v fail-err=%v", gid, failAck, failErr)
			http.Error(w, msg, 500)
			return
		}
		fmt.Fprint(w, "OK "+gid)
	})

	fmt.Println("saga-coordinator listen :7000")
	http.ListenAndServe(":7000", nil)
}
