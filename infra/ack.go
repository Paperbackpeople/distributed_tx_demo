package infra

import txv1 "distributed_tx_demo/api/tx/v1"

func OK() *txv1.Ack { return &txv1.Ack{Ok: true} }

func KO(err error) *txv1.Ack {
	return &txv1.Ack{
		Ok:  false,
		Msg: err.Error(),
	}
}
