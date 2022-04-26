package consensus

import (
	"sort"
	"xxx/contract"
	"xxx/types"
	"xxx/utils"
)

type indexTx struct {
	index int
	tx    *types.Tx
	ok    bool
}

type verifyTxTask struct {
	ch chan<- *indexTx
	tx *indexTx
}

func (t *verifyTxTask) Do() {
	if t.tx.tx.Verify() {
		t.tx.ok = true
	}
	t.ch <- t.tx
}

type preBlockTask struct {
	b     *types.Block
	pool  *utils.GoPool
	bch   chan<- *preBlock
	check bool
}

func (t *preBlockTask) Do() {
	indexTxs := make([]*indexTx, len(t.b.Txs))
	if !t.check || len(t.b.Txs) == 0 {
		for i, tx := range t.b.Txs {
			indexTxs[i] = &indexTx{tx: tx, index: i, ok: true}
		}
		t.bch <- &preBlock{Header: t.b.Header, txs: indexTxs, ok: true}
		return
	}

	txch := make(chan *indexTx, 1)
	go func() {
		for i, tx := range t.b.Txs {
			t.pool.Put(&verifyTxTask{ch: txch, tx: &indexTx{tx: tx, index: i}})
		}
	}()

	i := 0
	for tx := range txch {
		indexTxs[i] = tx
		i++
		if i == len(indexTxs) {
			break
		}
	}
	close(txch)

	sort.Slice(indexTxs, func(i, j int) bool {
		return indexTxs[i].index < indexTxs[j].index
	})
	clog.Infow("verify tx signature", "height", t.b.Header.Height, "ntxs", len(indexTxs))
	t.bch <- &preBlock{Header: t.b.Header, txs: indexTxs, ok: true}
}

type execTxResult struct {
	index int
	tx    *types.Tx
	ok    bool
}

type execTxTask struct {
	index int
	tx    *types.Tx
	cc    *contract.Container
	ch    chan *execTxResult
}

func (t *execTxTask) Do() {
	err := t.cc.ExecTx(t.tx)
	t.ch <- &execTxResult{index: t.index, tx: t.tx, ok: err == nil}
}

func (c *Consensus) execTxs(txs []*types.Tx) []*types.Tx {
	ch := make(chan *execTxResult, 16)
	defer close(ch)
	go func() {
		for i, tx := range txs {
			c.txsPool.Put(&execTxTask{index: i, tx: tx, cc: c.cc, ch: ch}, int(tx.Sig.PublicKey[0])%16)
		}
	}()
	var txResults []*execTxResult
	for tr := range ch {
		txResults = append(txResults, tr)
	}
	sort.Slice(txResults, func(i, j int) bool {
		return txResults[i].index < txResults[j].index
	})
	index := 0
	for _, tr := range txResults {
		if tr.ok {
			txs[index] = tr.tx
			index++
		}
	}
	txs = txs[:index]
	return txs
}
