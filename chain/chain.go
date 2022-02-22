package chain

import (
	"fmt"
	"sync"

	"xxx/config"
	"xxx/crypto"
	"xxx/db"
	"xxx/log"
	"xxx/p2p"
	"xxx/types"
)

var clog = log.New("chain")

type Chain struct {
	*config.Config

	mu          sync.Mutex
	curHeight   int64
	n           *p2p.Node
	db          db.DB
	preblock_mp map[int64]*types.Block
}

func New(conf *config.Config) (*Chain, error) {
	ldb, err := db.NewLDB(conf.DataPath)
	if err != nil {
		return nil, err
	}
	priv, err := crypto.PrivateKeyFromString(conf.Consensus.PrivateSeed)
	if err != nil {
		return nil, err
	}
	p2pConf := &p2p.Conf{Priv: priv, Port: conf.ServerPort, Topics: types.ChainTopics}
	node, err := p2p.NewNode(p2pConf)
	if err != nil {
		return nil, err
	}
	return &Chain{
		Config:      conf,
		db:          ldb,
		n:           node,
		preblock_mp: make(map[int64]*types.Block),
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
	c.mu.Lock()
	defer c.mu.Unlock()

	clog.Infow("handleNewBlock", "height", nb.Header.Height)
	if c.curHeight >= nb.Header.Height {
		return nil
	}

	pb := c.preblock_mp[nb.Header.Height]
	sb := &types.StoreBlock{Header: nb.Header}
	sb.TxHashs = append(sb.TxHashs, nb.Tx0.Hash())
	b := &types.Block{Header: nb.Header}

	for _, tx := range pb.Txs {
		th := tx.Hash()
		for _, h := range nb.FailedHashs {
			if string(th) == string(h) {
				continue
			}
		}
		sb.TxHashs = append(sb.TxHashs, th)
		b.Txs = append(b.Txs, tx)
	}

	_, err := c.writeTx(nb.Tx0)
	if err != nil {
		clog.Errorw("handleNewBlock error", "err", err, "height", nb.Header.Height)
		return err
	}

	err = c.writeBlock(b.Hash(), sb)
	if err != nil {
		clog.Errorw("handleNewBlock error", "err", err, "height", nb.Header.Height)
		return err
	}
	for _, h := range nb.FailedHashs {
		c.db.Delete(h)
	}

	delete(c.preblock_mp, c.curHeight)
	c.curHeight = nb.Header.Height

	npb := c.preblock_mp[c.curHeight+int64(c.Chain.PreBlocks)]
	npb.Header.TxsHash = types.TxsMerkel(pb.Txs)
	return nil
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
		_, err := c.writeTx(tx)
		if err != nil {
			clog.Errorw("handleTxs error", "err", err)
			return
		}

		c.mu.Lock()
		height := c.curHeight + int64(c.Chain.PreBlocks) //+ int64(th[0]%byte(c.Chain.ShardNum))
		b, ok := c.preblock_mp[height]
		if !ok {
			b = &types.Block{Header: &types.Header{Height: height}}
			c.preblock_mp[height] = b
		}
		b.Txs = append(b.Txs, tx)
		clog.Infow("handleTxs", "ntx", len(b.Txs), "height", height)
		c.mu.Unlock()
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
		b, ok := c.preblock_mp[i]
		if !ok {
			// return nil, fmt.Errorf("the %d preblock NOT here", i)
			break
		}
		bs = append(bs, b)
	}
	return &types.BlocksReply{Bs: bs, LastHeight: c.curHeight}, nil
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
