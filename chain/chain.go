package chain

import (
	"fmt"

	"xxx/crypto"
	"xxx/db"
	"xxx/p2p"
	"xxx/types"
)

type Conf struct {
	RpcAddr    string
	ServerAddr string
	DataPath   string
}

type Chain struct {
	*Conf
	curHeight int64
	n         *p2p.Node
	db        db.DB
	bm        map[int64]*types.Block
}

func New(conf *Conf, n *p2p.Node) (*Chain, error) {
	ldb, err := db.NewLDB(conf.DataPath)
	if err != nil {
		return nil, err
	}
	return &Chain{
		Conf: conf,
		db:   ldb,
		n:    n,
		bm:   make(map[int64]*types.Block),
	}, nil
}

func (c *Chain) handleP2pMsg(m *p2p.Msg) {
	switch m.Topic {
	case p2p.NewBlockTopic:
		var b types.NewBlock
		err := types.Unmarshal(m.Data, &b)
		if err != nil {
			panic(err)
		}
		c.handleNewBlock(&b)
	case p2p.GetBlocksTopic:
		var b types.GetBlocks
		err := types.Unmarshal(m.Data, &b)
		if err != nil {
			panic(err)
		}
		c.handleGetBlock(m.PID, &b)
	}
}

func (c *Chain) Run() {
	go runRpc(c.RpcAddr, c)
	for m := range c.n.C {
		c.handleP2pMsg(m)
	}
}

func (c *Chain) handleNewBlock(nb *types.NewBlock) {
	cb := c.bm[nb.Header.Height]
	sb := &types.StoreBlock{Header: nb.Header}
	sb.TxHashs = append(sb.TxHashs, nb.Tx0.Hash())
	b := &types.Block{Header: nb.Header}

	for _, tx := range cb.Txs {
		f := false
		th := tx.Hash()
		for _, h := range nb.FailedHashs {
			if string(th) == string(h) {
				f = true
			}
		}
		if !f {
			sb.TxHashs = append(sb.TxHashs, th)
			b.Txs = append(b.Txs, tx)
		}
	}

	_, err := c.writeTx(nb.Tx0)
	if err != nil {
		panic(err)
	}

	err = c.writeBlock(b.Hash(), sb)
	if err != nil {
		panic(err)
	}
	for _, h := range nb.FailedHashs {
		c.db.Delete(h)
	}

	delete(c.bm, c.curHeight)
	c.curHeight = nb.Header.Height

	pb := c.bm[c.curHeight+4]
	var hashs [][]byte
	for _, tx := range pb.Txs {
		hashs = append(hashs, tx.Hash())
	}
	pb.Header.TxsHash = crypto.Merkle(hashs)
	c.n.Publish(p2p.PreBlockTopic, pb)
}

func (c *Chain) writeBlock(hash []byte, sb *types.StoreBlock) error {
	val, err := types.Marshal(sb)
	if err != nil {
		return err
	}
	err = c.db.Set([]byte(fmt.Sprintf("%d", sb.Header.Height)), hash)
	if err != nil {
		return err
	}
	return c.db.Set(hash, val)
}

func (c *Chain) writeTx(tx *types.Tx) ([]byte, error) {
	val, err := types.Marshal(tx)
	if err != nil {
		return nil, err
	}

	th := tx.Hash()
	return th, c.db.Set(th, val)
}

func (c *Chain) handleTxs(txs []*types.Tx) {
	for _, tx := range txs {
		th, err := c.writeTx(tx)
		if err != nil {
			return
		}

		height := c.curHeight + 4 + int64(th[0]%8)
		b, ok := c.bm[height]
		if !ok {
			b = &types.Block{Header: &types.Header{Height: height}}
		}
		b.Txs = append(b.Txs, tx)
	}
}

func (c *Chain) GetBlockByHeight(height int64) (*types.Block, error) {
	hash, err := c.db.Get([]byte(fmt.Sprintf("%d", height)))
	if err != nil {
		return nil, err
	}
	return c.GetBlock(hash)
}

func (c *Chain) GetBlock(hash []byte) (*types.Block, error) {
	val, err := c.db.Get(hash)
	if err != nil {
		return nil, err
	}

	var sb types.StoreBlock
	err = types.Unmarshal(val, &sb)
	if err != nil {
		return nil, err
	}

	txs := make([]*types.Tx, len(sb.TxHashs))
	for i, h := range sb.TxHashs {
		tx, err := c.getTx(h)
		if err != nil {
			panic(err)
		}
		txs[i] = tx
	}
	return &types.Block{Header: sb.Header, Txs: txs}, nil
}

func (c *Chain) getTx(h []byte) (*types.Tx, error) {
	v, err := c.db.Get(h)
	if err != nil {
		return nil, err
	}
	tx := new(types.Tx)
	err = types.Unmarshal(v, tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (c *Chain) handleGetBlock(pid string, m *types.GetBlocks) {
	br, _ := c.getBlocks(m)
	c.n.SendMsg(pid, p2p.BlocksTopic, br)
}

func (c *Chain) getBlocks(m *types.GetBlocks) (*types.BlocksReply, error) {
	var bs []*types.Block
	for i := m.Start; i < m.Start+m.Count; i++ {
		b, err := c.GetBlockByHeight(i)
		if err != nil {
			break
		}
		bs = append(bs, b)
	}

	return &types.BlocksReply{Bs: bs, LastHeight: c.curHeight}, nil
}

// TODO: query tx by address
