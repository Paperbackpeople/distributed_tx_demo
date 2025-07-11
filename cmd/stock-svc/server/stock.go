package server

import (
	"context"
	"database/sql"
	"log"

	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
)

type Stock struct {
	txv1.UnimplementedStockSvcServer
}

func (s *Stock) Execute(ctx context.Context, in *txv1.StockSagaRequest) (*txv1.Ack, error) {
	log.Printf("[Stock.Execute] in: %+v", in)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	var cnt int
	_ = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_log WHERE gid=? AND status='CONFIRMED'", in.Gid).Scan(&cnt)
	if cnt == 1 {
		tx.Rollback()
		return infra.OK(), nil
	}

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

	_, err = tx.ExecContext(ctx,
		"UPDATE stock SET available=available-? WHERE product_id=?",
		in.Qty, in.ProductId)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO stock_log(gid,product_id,qty,status) VALUES(?,?,?,'CONFIRMED')",
		in.Gid, in.ProductId, in.Qty)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}

func (s *Stock) Compensate(ctx context.Context, in *txv1.StockSagaRequest) (*txv1.Ack, error) {
	log.Printf("[Stock.Compensate] in: %+v", in)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	var qty int32
	err = tx.QueryRowContext(ctx,
		"SELECT qty FROM stock_log WHERE gid=? AND status='CONFIRMED' FOR UPDATE", in.Gid).Scan(&qty)
	if err != nil {
		if err == sql.ErrNoRows {
			tx.Rollback()
			return infra.OK(), nil
		}
		tx.Rollback()
		return infra.KO(err), nil
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE stock SET available=available+? WHERE product_id=?", qty, in.ProductId)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE stock_log SET status='COMPENSATED' WHERE gid=?", in.Gid)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}
