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
	br, err := c.rpcGetBlocks(height, 1, "GetPreBlocks")
	if err != nil {
		clog.Infow("getPreBlocks err", "err", err, "height", height)
		return err
	}
	if len(br.Bs) == 0 {
		clog.Infow("getPreBlocks return 0 blocks")
		return nil
	}
	for _, b := range br.Bs {
		c.mch <- &types.PMsg{Msg: b, Topic: types.PreBlockTopic}
	}
	return nil
}

func (c *Consensus) newRpcClient() error {
	clt, err := c.getRpcClient(c.DataNode, "Chain")
	if err != nil {
		clog.Error("handleBlocksReply error", "err", err)
		return err
	}
	c.rpcClt = clt
	return nil
}

func (c *Consensus) getBlocks(start, count int64) error {
	br, err := c.rpcGetBlocks(start, count, "GetBlocks")
	if err != nil {
		clog.Infow("getPreBlocks err", "err", err, "height", start)
		return err
	}
	if len(br.Bs) == 0 {
		clog.Infow("getPreBlocks return 0 blocks")
		return nil
	}
	c.mch <- &types.PMsg{Msg: br, Topic: types.BlockTopic}
	return nil
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
		clog.Error("handleBlocksReply error", "err", err)
		return nil, err
	}
	return &br, nil
}
