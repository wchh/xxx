package chain

import (
	"context"
	"xxx/types"

	"github.com/smallnest/rpcx/server"
)

func (c *Chain) GetPreBlocks(ctx context.Context, args *types.GetBlocks, reply *types.BlocksReply) error {
	br, err := c.getPreBlocks(args)
	if err != nil {
		return err
	}
	*reply = *br
	return nil
}

func (c *Chain) GetBlocks(ctx context.Context, args *types.GetBlocks, reply *types.BlocksReply) error {
	br, err := c.getBlocks(args)
	if err != nil {
		return err
	}
	*reply = *br
	return nil
}

func (c *Chain) SendTx(ctx context.Context, args *types.Tx, reply *struct{}) error {
	c.handleTxs([]*types.Tx{args})
	return nil
}

func (c *Chain) SendTxs(ctx context.Context, args []*types.Tx, reply *struct{}) error {
	c.handleTxs(args)
	return nil
}

func (c *Chain) QueryTx(ctx context.Context, args []byte, reply *types.Tx) error {
	tx, err := c.getTx(args)
	if err != nil {
		return err
	}
	*reply = *tx
	return nil
}

func (c *Chain) QueryBlock(ctx context.Context, args []byte, reply *types.Block) error {
	b, err := c.GetBlock(args)
	if err != nil {
		return err
	}
	*reply = *b
	return nil
}

func (c *Chain) PeersInfo(ctx context.Context, args *struct{}, reply *types.PeersInfo) error {
	return nil
}

func (c *Chain) ServerInfo(ctx context.Context, args *struct{}, reply *types.ServerInfo) error {
	return nil
}

func (c *Chain) QueryBlockByHeight(ctx context.Context, args int64, reply *types.Block) error {
	b, err := c.GetBlockByHeight(args)
	if err != nil {
		return err
	}
	*reply = *b
	return nil
}

func runRpc(addr string, c *Chain) {
	s := server.NewServer()
	s.RegisterName("Chain", c, "")
	go s.Serve("tcp", addr)
}
