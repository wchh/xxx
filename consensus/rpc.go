package consensus

import (
	"context"
	"xxx/contract/coin"
	"xxx/contract/ycc"
	"xxx/types"

	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/server"
)

func (c *Consensus) LastBlock(ctx context.Context, args *struct{}, reply *types.Block) error {
	*reply = *c.lastBlock
	return nil
}

func (c *Consensus) QueryDeposit(ctx context.Context, args string, reply *int64) error {
	b, err := ycc.QueryDeposit(args, c.db)
	if err != nil {
		return err
	}
	*reply = b
	return nil
}

func (c *Consensus) QueryBalance(ctx context.Context, args string, reply *int64) error {
	b, err := coin.QueryBalance(args, c.db)
	if err != nil {
		return err
	}
	*reply = b
	return nil
}

func (c *Consensus) QueryIssue(ctx context.Context, args *struct{}, reply *int64) error {
	b, err := coin.QueryIssueAll(c.db)
	if err != nil {
		return err
	}
	*reply = b
	return nil
}

func (c *Consensus) QueryVotePrice(ctx context.Context, args *struct{}, reply *float64) error {
	*reply = c.VotePrice
	return nil
}

func runRpc(addr string, c *Consensus) {
	s := server.NewServer()
	s.RegisterName("Consensus", c, "")
	go s.Serve("tcp", addr)
}

func (c *Consensus) getRpcClient(addr, svc string) (client.XClient, error) {
	d, err := client.NewPeer2PeerDiscovery("tcp@"+addr, "")
	if err != nil {
		return nil, err
	}
	opt := client.DefaultOption
	// opt.SerializeType = protocol.JSON
	return client.NewXClient(svc, client.Failtry, client.RandomSelect, d, opt), nil
}

func (c *Consensus) getPreBlocks(height int64) error {
	if c.UseRpcToDataNode {
		c.pool.Put(&getPreBlockTask{height: height, c: c})
	} else {
		c.p2pGetBlocks(height, 1, types.GetPreBlockTopic)
	}
	return nil
}

func (c *Consensus) connectDatanode() error {
	clt, err := c.getRpcClient(c.DataNodeRpc, "Chain")
	if err != nil {
		clog.Error("connectDatanode error", "err", err)
		return err
	}
	clog.Infow("connect datanode", "datanode", c.DataNodeRpc)
	c.rpcClt = clt
	return nil
}

func (c *Consensus) getBlocks(start, count int64) error {
	if c.UseRpcToDataNode {
		c.pool.Put(&getBlocksTask{start: start, count: int(count), c: c})
	} else {
		c.p2pGetBlocks(start, count, types.GetBlocksTopic)
	}
	return nil
}

func (c *Consensus) setNewBlock(nb *types.NewBlock) error {
	if c.UseRpcToDataNode {
		return c.rpcClt.Call(context.Background(), "SetNewBlock", nb, nil)
	} else {
		return c.node.Send(c.DataNodePID, types.SetNewBlockTopic, nb)
	}
}

func (c *Consensus) p2pGetBlocks(start, count int64, topic string) error {
	arg := &types.GetBlocks{
		Start: start,
		Count: count,
	}
	return c.node.Send(c.DataNodePID, topic, arg)
}

func (c *Consensus) rpcGetBlocks(start, count int64, m string) (*types.BlocksReply, error) {
	clt := c.rpcClt
	r := &types.GetBlocks{
		Start: start,
		Count: count,
	}
	var br types.BlocksReply
	err := clt.Call(context.Background(), m, r, &br)
	if err != nil {
		clog.Errorw("rpcGetBlocks error", "err", err, "method", m)
		return nil, err
	}
	return &br, nil
}

type getBlocksTask struct {
	start int64
	count int
	c     *Consensus
}

func (t *getBlocksTask) Do() {
	c := t.c
	br, err := c.rpcGetBlocks(t.start, int64(t.count), "GetPreBlocks")
	if err != nil {
		clog.Infow("getPreBlocks err", "err", err, "height", t.start)
		return
	}
	if len(br.Bs) == 0 {
		clog.Infow("getPreBlocks return 0 blocks", "height", t.start)
		return
	}
	c.mch <- &types.PMsg{Msg: br, Topic: types.BlocksTopic}
}

type getPreBlockTask struct {
	height int64
	c      *Consensus
}

func (t *getPreBlockTask) Do() {
	c := t.c
	height := t.height
	br, err := c.rpcGetBlocks(height, 1, "GetPreBlocks")
	if err != nil {
		clog.Infow("getPreBlocks err", "err", err, "height", height)
		return
	}
	if len(br.Bs) == 0 {
		clog.Infow("getPreBlocks return 0 blocks", "height", height)
		return
	}
	for _, b := range br.Bs {
		c.mch <- &types.PMsg{Msg: b, Topic: types.PreBlockTopic}
	}
}
