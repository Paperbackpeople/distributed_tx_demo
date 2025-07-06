package server

import (
	"context"
	"database/sql"

	txv1 "distributed_tx_demo/api/tx/v1"
	"distributed_tx_demo/infra"
)

type Pay struct {
	txv1.UnimplementedPaySvcServer
}

// Try：冻结余额，写 payment
func (p *Pay) Try(ctx context.Context, in *txv1.PayTry) (*txv1.Ack, error) {
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	// 幂等校验：如果已存在该gid的payment记录直接OK
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
	if bal < int(in.Amount) {
		tx.Rollback()
		return &txv1.Ack{Ok: false, Msg: "insufficient balance"}, nil
	}

	// 冻结余额
	_, err = tx.ExecContext(ctx, "UPDATE account SET balance=balance-?, reserved=reserved+? WHERE user_id=?", in.Amount, in.Amount, in.UserId)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	// 写 payment
	_, err = tx.ExecContext(ctx, "INSERT INTO payment(gid,user_id,amount,status) VALUES(?,?,?,'PENDING')", in.Gid, in.UserId, in.Amount)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}

// Confirm：正式扣除 reserved，支付状态CONFIRMED
func (p *Pay) Confirm(ctx context.Context, gid *txv1.Gid) (*txv1.Ack, error) {
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	// 查询 payment
	var userID int64
	var amount int
	err = tx.QueryRowContext(ctx, "SELECT user_id,amount FROM payment WHERE gid=? AND status='PENDING'", gid.Gid).Scan(&userID, &amount)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return infra.OK(), nil // 已被确认或回滚，无需重复
		}
		return infra.KO(err), nil
	}

	// 扣除 reserved
	_, err = tx.ExecContext(ctx, "UPDATE account SET reserved=reserved-? WHERE user_id=?", amount, userID)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	// 更新支付状态
	_, err = tx.ExecContext(ctx, "UPDATE payment SET status='CONFIRMED' WHERE gid=? AND status='PENDING'", gid.Gid)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}

// Cancel：回滚 reserved，支付状态REFUNDED
func (p *Pay) Cancel(ctx context.Context, gid *txv1.Gid) (*txv1.Ack, error) {
	db := infra.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return infra.KO(err), nil
	}

	// 查询 payment
	var userID int64
	var amount int
	err = tx.QueryRowContext(ctx, "SELECT user_id,amount FROM payment WHERE gid=? AND status='PENDING'", gid.Gid).Scan(&userID, &amount)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return infra.OK(), nil // 已被确认或回滚，无需重复
		}
		return infra.KO(err), nil
	}

	// 回滚余额与 reserved
	_, err = tx.ExecContext(ctx, "UPDATE account SET balance=balance+?, reserved=reserved-? WHERE user_id=?", amount, amount, userID)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	// 更新支付状态
	_, err = tx.ExecContext(ctx, "UPDATE payment SET status='REFUNDED' WHERE gid=? AND status='PENDING'", gid.Gid)
	if err != nil {
		tx.Rollback()
		return infra.KO(err), nil
	}

	return infra.OK(), tx.Commit()
}
