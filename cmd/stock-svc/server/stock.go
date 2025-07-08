// cmd/stock-svc/server/stock.go
package server

import (
	"context"
	"database/sql"
	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
	"fmt"
	"log"
	"time"
)

type Stock struct {
	txv1.UnimplementedStockSvcServer
}

func (s *Stock) Try(ctx context.Context, in *txv1.StockTry) (*txv1.Ack, error) {
	log.Printf("[Stock.Try] in: %+v", in)
	lockKey := fmt.Sprintf("lock:stock:%d", in.ProductId)
	token, ok, err := infra.Acquire(ctx, lockKey, 3*time.Second)
	if err != nil {
		return infra.KO(err), nil
	}
	if !ok {
		return &txv1.Ack{Ok: false, Msg: "busy, try again"}, nil
	}
	defer infra.Release(context.Background(), lockKey, token)

	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	// 幂等
	var dummy int
	_ = tx.QueryRowContext(ctx,
		"SELECT 1 FROM stock_log WHERE gid=? LIMIT 1", in.Gid).Scan(&dummy)
	if dummy == 1 {
		tx.Rollback()
		return infra.OK(), nil
	}

	// 校验库存
	var avail int
	if err := tx.QueryRowContext(ctx,
		"SELECT available FROM stock WHERE product_id=? FOR UPDATE", in.ProductId).Scan(&avail); err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}
	if avail < int(in.Qty) {
		tx.Rollback()
		return &txv1.Ack{Ok: false, Msg: "not enough stock"}, nil
	}

	// 冻结
	_, err = tx.ExecContext(ctx,
		"UPDATE stock SET available=available-?, reserved=reserved+? WHERE product_id=?",
		in.Qty, in.Qty, in.ProductId)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	// 写 log
	_, err = tx.ExecContext(ctx,
		"INSERT INTO stock_log(gid,product_id,qty,status) VALUES(?,?,?,'RESERVED')",
		in.Gid, in.ProductId, in.Qty)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}
func (s *Stock) Confirm(ctx context.Context, gid *txv1.Gid) (*txv1.Ack, error) {
	log.Printf("[Stock.Confirm] gid: %s", gid.Gid)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}
	defer tx.Rollback()

	// 查找 log 拿 product_id/qty
	var productID int64
	var qty int32
	err = tx.QueryRowContext(ctx,
		"SELECT product_id, qty FROM stock_log WHERE gid=? AND status='RESERVED' FOR UPDATE", gid.Gid).Scan(&productID, &qty)
	if err != nil {
		if err == sql.ErrNoRows {
			return infra.OK(), nil
		}
		return infra.KO(err), nil
	}

	// RESERVED -> CONFIRMED
	_, err = tx.ExecContext(ctx,
		"UPDATE stock_log SET status='CONFIRMED' WHERE gid=? AND status='RESERVED'", gid.Gid)
	if err != nil {
		return infra.KO(err), nil
	}

	// reserved -= qty
	_, err = tx.ExecContext(ctx,
		"UPDATE stock SET reserved=reserved-? WHERE product_id=?", qty, productID)
	if err != nil {
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}

func (s *Stock) Cancel(ctx context.Context, gid *txv1.Gid) (*txv1.Ack, error) {
	log.Printf("[Stock.Cancel] gid: %s", gid.Gid)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}
	defer tx.Rollback()

	// 查找 log
	var productID int64
	var qty int32
	err = tx.QueryRowContext(ctx,
		"SELECT product_id, qty FROM stock_log WHERE gid=? AND status='RESERVED' FOR UPDATE", gid.Gid).Scan(&productID, &qty)
	if err != nil {
		if err == sql.ErrNoRows {
			return infra.OK(), nil // 重复 Cancel
		}
		return infra.KO(err), nil
	}

	// RESERVED -> CANCELED
	_, err = tx.ExecContext(ctx,
		"UPDATE stock_log SET status='CANCELED' WHERE gid=? AND status='RESERVED'", gid.Gid)
	if err != nil {
		return infra.KO(err), nil
	}

	// 回滚库存: available += qty, reserved -= qty
	_, err = tx.ExecContext(ctx,
		"UPDATE stock SET available=available+?, reserved=reserved-? WHERE product_id=?", qty, qty, productID)
	if err != nil {
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}
