package server

import (
	"context"
	"database/sql"

	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
	"log"
)

type Pay struct {
	txv1.UnimplementedPaySvcServer
}

func (p *Pay) Execute(ctx context.Context, in *txv1.PaySagaRequest) (*txv1.Ack, error) {
	log.Printf("[Pay.Execute] in: %+v", in)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	var dummy int
	_ = tx.QueryRowContext(ctx, "SELECT 1 FROM payment WHERE gid=? LIMIT 1", in.Gid).Scan(&dummy)
	if dummy == 1 {
		tx.Rollback()
		return infra.OK(), nil
	}

	// 检查余额
	var bal, res int
	err = tx.QueryRowContext(ctx, "SELECT balance,reserved FROM account WHERE user_id=? FOR UPDATE", in.UserId).Scan(&bal, &res)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}
	if bal < int(in.TotalAmt) {
		tx.Rollback()
		return &txv1.Ack{Ok: false, Msg: "insufficient balance"}, nil
	}

	// 直接扣款
	_, err = tx.ExecContext(ctx, "UPDATE account SET balance=balance-? WHERE user_id=?", in.TotalAmt, in.UserId)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	// 写 payment
	_, err = tx.ExecContext(ctx, "INSERT INTO payment(gid,user_id,amount,status) VALUES(?,?,?,'CONFIRMED')", in.Gid, in.UserId, in.TotalAmt)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}

func (p *Pay) Compensate(ctx context.Context, in *txv1.PaySagaRequest) (*txv1.Ack, error) {
	log.Printf("[Pay.Compensate] in: %+v", in)
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	// 查询 payment
	var status string
	err = tx.QueryRowContext(ctx, "SELECT status FROM payment WHERE gid=? ", in.Gid).Scan(&status)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return infra.OK(), nil // 已被确认或回滚，无需重复
		}
		return infra.KO(err), nil
	}
	if status != "CONFIRMED" {
		tx.Rollback()
		return infra.OK(), nil
	}
	// 回滚余额
	_, err = tx.ExecContext(ctx, "UPDATE account SET balance=balance+? WHERE user_id=?", in.TotalAmt, in.UserId)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	// 更新 payment 状态
	_, err = tx.ExecContext(ctx, "UPDATE payment SET status='COMPENSATED' WHERE gid=? ", in.Gid)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}
