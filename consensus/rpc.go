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

func (c *Consensus) QueryVotePrice(ctx context.Context, args *struct{}, reply *int64) error {
	*reply = c.VotePrice
	return nil
}

func runRpc(addr string, c *Consensus) {
	s := server.NewServer()
	// s.Register(c, "")
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
