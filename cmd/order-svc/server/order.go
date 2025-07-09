package server

import (
	"context"
	"database/sql"
	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
	"log"
)

type Order struct {
	txv1.UnimplementedOrderSvcServer
}

func (o *Order) Execute(ctx context.Context, in *txv1.OrderSagaRequest) (*txv1.Ack, error) {
	log.Printf("[Order.Execute] in: %+v", in)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	// Check if the order already exists
	var cnt int
	_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE gid = ?", in.Gid).Scan(&cnt)
	log.Printf("[Order.Execute] existing order count: %d", cnt)
	if cnt == 1 {
		tx.Rollback()
		return infra.OK(), nil
	}

	// Insert the order
	_, err = tx.ExecContext(ctx, "INSERT INTO orders (gid, user_id,total_amt, status) VALUES (?, ?, ?, 'PENDING')",
		in.Gid, in.UserId, in.TotalAmt)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	//Get the order ID
	var oid int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM orders WHERE gid = ?", in.Gid).Scan(&oid)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	// Insert the order items
	stmt, _ := tx.PrepareContext(ctx, "INSERT INTO order_item (order_id, product_id, qty, price) VALUES (?, ?, ?, ?)")
	for _, item := range in.Items {
		_, err = stmt.ExecContext(ctx, oid, item.ProductId, item.Qty, item.Price)
		if err != nil {
			tx.Rollback()
			return infra.KO(err), nil
		}
	}
	stmt.Close()
	return infra.OK(), tx.Commit()
}

func (o *Order) Compensate(ctx context.Context, in *txv1.OrderSagaRequest) (*txv1.Ack, error) {
	log.Printf("[Order.Compensate] in: %+v", in)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}
	
	var oid int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM orders WHERE gid = ?", in.Gid).Scan(&oid)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return infra.OK(), nil
		}
		return infra.KO(err), nil
	}

	// Delete the order items
	_, err = tx.ExecContext(ctx, "DELETE FROM order_item WHERE order_id = ?", oid)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM orders WHERE id = ?", oid)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}
	return infra.OK(), tx.Commit()
}


