package server

import (
	"context"
	"database/sql"
	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
)

type Order struct {
	txv1.UnimplementedOrderSvcServer
}

func (o *Order) Try(ctx context.Context, in *txv1.OrderTry) (*txv1.Ack, error) {
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	// Check if the order already exists
	var cnt int
	_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE gid = ?", in.Gid).Scan(&cnt)
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

func ack(err error) (*txv1.Ack, error) {
	if err != nil && err != sql.ErrNoRows {
		return infra.KO(err), nil
	}
	return infra.OK(), nil
}

func (o *Order) Confirm(ctx context.Context, gid *txv1.Gid) (*txv1.Ack, error) {
	_, err := infra.DB().ExecContext(ctx,
		"UPDATE orders SET status='CONFIRMED' WHERE gid=? AND status='PENDING'", gid.Gid)
	return ack(err)
}
func (o *Order) Cancel(ctx context.Context, gid *txv1.Gid) (*txv1.Ack, error) {
	_, err := infra.DB().ExecContext(ctx,
		"UPDATE orders SET status='CANCELED' WHERE gid=? AND status='PENDING'", gid.Gid)
	return ack(err)
}
