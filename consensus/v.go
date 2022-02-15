package consensus

import (
	"errors"
	"xxx/types"
)

type taskResult struct {
	tx *types.Tx
	ok bool
}
type txVerifyTask struct {
	tx *types.Tx
	ch chan *taskResult
}

func (t *txVerifyTask) Do() {
	t.ch <- &taskResult{tx: t.tx, ok: t.tx.Verify()}
}

func (c *Consensus) txsVerifySig(txs []*types.Tx, cpuNum int, errReturn bool) ([]*types.Tx, [][]byte, error) {
	ch := make(chan *taskResult) // 任务处理结果
	defer close(ch)

	done := make(chan struct{}) // 如果有错误，就终止发送任务
	ich := make(chan int)       // 指示发送了多少任务
	pool := c.pool

	go func() {
		defer close(ich)
		for i, tx := range txs {
			pool.Put(&txVerifyTask{tx, ch})
			select {
			case <-done:
				ich <- i + 1
				return
			default:
			}
		}
		ich <- len(txs)
	}()

	var okTxs []*types.Tx
	var faildHashs [][]byte
	var err error
	k := 0
	j := 0

	for {
		vr := <-ch
		if vr.ok {
			okTxs = append(okTxs, vr.tx)
		} else {
			if errReturn {
				err = errors.New("tx signature verify failed")
				close(done)
			}
			faildHashs = append(faildHashs, vr.tx.Hash())
		}

		//k 用于指示接收了多少结果
		//当发送的任务和接收的结果相同时，退出循环，这时才可以关闭 ch
		k++
		select {
		case j = <-ich:
		default:
		}
		if j == k {
			break
		}
	}
	if err == nil {
		close(done)
	}
	return okTxs, faildHashs, err
}
