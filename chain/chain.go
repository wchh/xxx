package chain

import (
	"fmt"
	"sync"

	"xxx/config"
	"xxx/db"
	"xxx/log"
	"xxx/types"
)

var clog = log.New("chain")

type preBlock struct {
	mu sync.Mutex
	b  *types.Block
}

func (pb *preBlock) txsLen() int {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	return len(pb.b.Txs)
}

func (pb *preBlock) merkel() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.b.Header.TxsHash = types.TxsMerkel(pb.b.Txs)
}

func (pb *preBlock) setTxs(txs []*types.Tx) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.b.Txs = append(pb.b.Txs, txs...)
}

func (pb *preBlock) handleNewBlock(nb *types.NewBlock) *types.StoreBlock {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	sb := &types.StoreBlock{Header: nb.Header}
	sb.TxHashs = [][]byte{nb.Tx0.Hash()}
	for _, tx := range pb.b.Txs {
		th := tx.Hash()
		for _, h := range nb.FailedHashs {
			if string(th) == string(h) {
				continue
			}
		}
		sb.TxHashs = append(sb.TxHashs, th)
	}
	return sb
}

type Chain struct {
	*config.DataNodeConfig

	mu        sync.Mutex
	curHeight int64
	// n           *p2p.Node
	db          db.DB
	preblock_mp map[int64]*preBlock
}

func New(conf *config.DataNodeConfig) (*Chain, error) {
	ldb, err := db.NewLDB(conf.DataPath)
	if err != nil {
		return nil, err
	}
	// priv, err := crypto.PrivateKeyFromString(conf.PrivateSeed)
	// if err != nil {
	// 	return nil, err
	// }
	// p2pConf := &p2p.Conf{Priv: priv, Port: conf.RpcPort, Topics: types.ChainTopics}
	// node, err := p2p.NewNode(p2pConf)
	// if err != nil {
	// 	return nil, err
	// }
	return &Chain{
		DataNodeConfig: conf,
		db:             ldb,
		// n:              node,
		preblock_mp: make(map[int64]*preBlock),
	}, nil
}

// func (c *Chain) handleP2pMsg(m *p2p.Msg) {
// 	switch m.Topic {
// case p2p.NewBlockTopic:
// 	var b types.NewBlock
// 	err := types.Unmarshal(m.Data, &b)
// 	if err != nil {
// 		panic(err)
// 	}
// 	c.handleNewBlock(&b)
// case p2p.GetPreBlocksTopic:
// 	var b types.GetBlocks
// 	err := types.Unmarshal(m.Data, &b)
// 	if err != nil {
// 		panic(err)
// 	}
// 	c.handleGetPreBlocks(m.PID, &b)
// case p2p.GetBlocksTopic:
// 	var b types.GetBlocks
// 	err := types.Unmarshal(m.Data, &b)
// 	if err != nil {
// 		panic(err)
// 	}
// 	c.handleGetBlocks(m.PID, &b)
// }
// }

func (c *Chain) Run() {
	runRpc(fmt.Sprintf(":%d", c.RpcPort), c)
	// for m := range c.n.C {
	// 	c.handleP2pMsg(m)
	// }
}

func (c *Chain) handleNewBlock(nb *types.NewBlock) error {
	clog.Infow("handleNewBlock", "height", nb.Header.Height)
	if c.curHeight >= nb.Header.Height {
		return nil
	}

	_, err := c.writeTx(nb.Tx0)
	if err != nil {
		clog.Errorw("handleNewBlock error", "err", err, "height", nb.Header.Height)
		return err
	}

	var sb *types.StoreBlock
	c.mu.Lock()
	pb, ok := c.preblock_mp[nb.Header.Height]
	c.mu.Unlock()
	if ok {
		sb = pb.handleNewBlock(nb)
	} else {
		sb = &types.StoreBlock{
			Header:  nb.Header,
			TxHashs: [][]byte{nb.Tx0.Hash()},
		}
	}

	err = c.writeBlock(sb)
	if err != nil {
		clog.Errorw("handleNewBlock error", "err", err, "height", nb.Header.Height)
		return err
	}
	for _, h := range nb.FailedHashs {
		c.db.Delete(h)
	}

	c.mu.Lock()
	c.curHeight = nb.Header.Height
	delete(c.preblock_mp, c.curHeight)
	npb, ok := c.preblock_mp[c.curHeight+int64(c.PreBlocks)]
	c.mu.Unlock()
	if ok {
		npb.merkel()
	}
	return nil
}

func (c *Chain) writeBlock(sb *types.StoreBlock) error {
	val, err := types.Marshal(sb)
	if err != nil {
		return err
	}
	bh := sb.Header.Hash()
	err = c.db.Set([]byte(fmt.Sprintf("%d", sb.Header.Height)), bh)
	if err != nil {
		return err
	}
	return c.db.Set(bh, val)
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
		_, err := c.writeTx(tx)
		if err != nil {
			clog.Errorw("handleTxs error", "err", err)
			return
		}

	}
	c.mu.Lock()
	height := c.curHeight + int64(c.PreBlocks) //+ int64(th[0]%byte(c.Chain.ShardNum))
	pb, ok := c.preblock_mp[height]
	if !ok {
		b := &types.Block{Header: &types.Header{Height: height}}
		pb = &preBlock{b: b}
		c.preblock_mp[height] = pb
	}
	c.mu.Unlock()
	pb.setTxs(txs)
	clog.Infow("handleTxs", "ntx", pb.txsLen(), "height", height)
}

func (c *Chain) getBlockByHeight(height int64) (*types.Block, error) {
	hash, err := c.db.Get([]byte(fmt.Sprintf("%d", height)))
	if err != nil {
		return nil, err
	}
	return c.getBlock(hash)
}

func (c *Chain) getBlock(hash []byte) (*types.Block, error) {
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

// func (c *Chain) handleGetPreBlocks(pid string, m *types.GetBlocks) {
// 	br, _ := c.getPreBlocks(m)
// 	c.n.Send(pid, p2p.PreBlocksReplyTopic, br)
// }

// func (c *Chain) handleGetBlocks(pid string, m *types.GetBlocks) {
// 	br, _ := c.getBlocks(m)
// 	c.n.Send(pid, p2p.BlocksReplyTopic, br)
// }

func (c *Chain) getPreBlocks(m *types.GetBlocks) (*types.BlocksReply, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var bs []*types.Block
	for i := m.Start; i < m.Start+m.Count; i++ {
		pb, ok := c.preblock_mp[i]
		if !ok {
			// return nil, fmt.Errorf("the %d preblock NOT here", i)
			break
		}
		bs = append(bs, pb.b)
	}
	clog.Infow("getPreBlock", "start", m.Start, "count", m.Count, "nb", len(bs))
	return &types.BlocksReply{Bs: bs, LastHeight: c.curHeight}, nil
}

func (c *Chain) getBlocks(m *types.GetBlocks) (*types.BlocksReply, error) {
	var bs []*types.Block
	for i := m.Start; i < m.Start+m.Count; i++ {
		b, err := c.getBlockByHeight(i)
		if err != nil {
			break
		}
		bs = append(bs, b)
	}

	clog.Infow("getBlock", "start", m.Start, "count", m.Count, "nb", len(bs))
	return &types.BlocksReply{Bs: bs, LastHeight: c.curHeight}, nil
}

// TODO: query tx by address
