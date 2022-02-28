package main

//lint:file-ignore U1000 Ignore all unused code

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"xxx/contract/coin"
	"xxx/contract/ycc"
	"xxx/crypto"
	"xxx/types"
	"xxx/utils"

	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/protocol"
	"github.com/urfave/cli/v2"
)

var xclient client.XClient

func main() {
	var rpcAddr string
	var service string
	if xclient != nil {
		defer xclient.Close()
	}

	app := &cli.App{
		Name:  "xxx-cli",
		Usage: "cli to xxx",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "rpcaddr",
				Aliases:     []string{"r"},
				Value:       "localhost:10801",
				Usage:       "rpc server address",
				Destination: &rpcAddr,
			},
			&cli.StringFlag{
				Name:        "service",
				Aliases:     []string{"s"},
				Value:       "Chain",
				Usage:       "Chain or Consensus rpc service",
				Destination: &service,
			},
		},
		Before: func(c *cli.Context) error {
			d, err := client.NewPeer2PeerDiscovery("tcp@"+rpcAddr, "")
			if err != nil {
				panic(err)
			}
			opt := client.DefaultOption
			opt.SerializeType = protocol.JSON
			if service != "Chain" {
				service = "Consensus"
			}
			xclient = client.NewXClient(service, client.Failtry, client.RandomSelect, d, opt)
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "test2",
				Usage: "run send txs test",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "n",
						Value: 30000,
					},
				},
				Action: func(c *cli.Context) error {
					test2(c.Int("n"))
					return nil
				},
			},
			{
				Name:  "test1",
				Usage: "run send txs test",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "n",
						Value: 30000,
					},
				},
				Action: func(c *cli.Context) error {
					test1(c.Int("n"))
					return nil
				},
			},
			{
				Name:  "test",
				Usage: "run send txs test",
				Action: func(c *cli.Context) error {
					runSendTx()
					return nil
				},
			},
			{
				Name:  "key",
				Usage: "gen 25519 key pair and address",
				Action: func(c *cli.Context) error {
					sk, err := crypto.NewKey()
					if err != nil {
						return err
					}
					fmt.Println("sk:", sk)
					fmt.Println("pk:", sk.PublicKey())
					fmt.Println("address:", sk.PublicKey().Address())
					return nil
				},
			},
			{
				Name:  "block",
				Usage: "query block by hash or by height",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "hash",
						Aliases: []string{"s"},
						Usage:   "block hash",
					},
					&cli.Int64Flag{
						Name:    "height",
						Aliases: []string{"g"},
						Usage:   "block height",
					},
				},
				Action: func(c *cli.Context) error {
					if c.NumFlags() == 0 {
						return errors.New("hash or height must be required")
					}
					return query_block(c.String("hash"), c.Int64("height"))
				},
			},
			{
				Name:  "last_block",
				Usage: "get last block",
				Action: func(c *cli.Context) error {
					return lastBlock()
				},
			},
			{
				Name:  "tx",
				Usage: "query tx by hash",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "hash",
						Aliases:  []string{"s"},
						Usage:    "block hash",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					return query_tx(c.String("hash"))
				},
			},
			{
				Name:  "coin",
				Usage: "coin contract commands",
				Subcommands: []*cli.Command{
					{
						Name: "transfer",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "sk",
								Aliases:  []string{"k"},
								Usage:    "privatekey for sign tx",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "to",
								Aliases:  []string{"t"},
								Usage:    "address to transfer",
								Required: true,
							},
							&cli.Int64Flag{
								Name:     "amount",
								Aliases:  []string{"a"},
								Usage:    "amount to transfer",
								Required: true,
							},
							&cli.Int64Flag{
								Name:    "height",
								Aliases: []string{"h"},
								Usage:   "tx in the block height",
							},
						},
						Action: func(c *cli.Context) error {
							sk := c.String("sk")
							to := c.String("to")
							amount := c.Int64("amount")
							height := c.Int64("height")
							return transfer(sk, to, amount, height)
						},
					},
					{
						Name:  "balance",
						Usage: "query addr balance",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "addr",
								Aliases:  []string{"a"},
								Usage:    "query balance of address",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							_, err := query_balance(c.String("addr"))
							return err
						},
					},
				},
			},
			{
				Name:  "ycc",
				Usage: "ycc contract commands",
				Subcommands: []*cli.Command{
					{
						Name:  "deposit",
						Usage: "deposit some ycc for minding",
						Flags: []cli.Flag{
							&cli.Int64Flag{
								Name:     "amount",
								Usage:    "amount of vote",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "sk",
								Aliases:  []string{"k"},
								Usage:    "privatekey for sign tx",
								Required: true,
							},
							&cli.StringFlag{
								Name:    "to",
								Aliases: []string{"t"},
								Usage:   "address to transfer",
								// Required: true,
							},
							&cli.Int64Flag{
								Name:    "height",
								Aliases: []string{"h"},
								Usage:   "tx in the block height",
							},
						},
						Action: func(c *cli.Context) error {
							sk := c.String("sk")
							to := c.String("to")
							amount := c.Int64("amount")
							height := c.Int64("height")
							return deposit(sk, to, amount, height)
						},
					},
					{
						Name:  "query",
						Usage: "query deposit of addr",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "addr",
								Aliases:  []string{"a"},
								Usage:    "query balance of address",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							_, err := query_deposit(c.String("addr"))
							return err
						},
					},
					{
						Name: "withdraw",
						Flags: []cli.Flag{
							&cli.Int64Flag{
								Name:     "amount",
								Usage:    "amount of vote",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "sk",
								Aliases:  []string{"k"},
								Usage:    "privatekey for sign tx",
								Required: true,
							},
							&cli.Int64Flag{
								Name:    "height",
								Aliases: []string{"h"},
								Usage:   "tx in the block height",
							},
						},
						Action: func(c *cli.Context) error {
							sk := c.String("sk")
							amount := c.Int64("amount")
							height := c.Int64("height")
							return withdraw(sk, amount, height)
						},
					},
					{
						Name:  "withdraw_all",
						Usage: "withdraw all deposit ycc",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "sk",
								Aliases:  []string{"k"},
								Usage:    "privatekey for sign tx",
								Required: true,
							},
							&cli.Int64Flag{
								Name:    "height",
								Aliases: []string{"h"},
								Usage:   "tx in the block height",
							},
						},
						Action: func(c *cli.Context) error {
							sk := c.String("sk")
							height := c.Int64("height")
							return withdraw_all(sk, height)
						},
					},
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func transfer(sk, to string, amount, height int64) error {
	priv, err := crypto.PrivateKeyFromString(sk)
	if err != nil {
		return err
	}
	tx, err := coin.CreateTransferTx(priv, to, amount, height)
	if err != nil {
		return err
	}
	println(tx)
	return xclient.Call(context.Background(), "SendTx", tx, nil)
}

func deposit(sk, to string, amount, height int64) error {
	priv, err := crypto.PrivateKeyFromString(sk)
	if err != nil {
		return err
	}
	tx, err := ycc.CreateDepositTx(priv, to, amount, height)
	if err != nil {
		return err
	}
	println(tx)
	return xclient.Call(context.Background(), "SendTx", tx, nil)
}

func withdraw(sk string, amount, height int64) error {
	priv, err := crypto.PrivateKeyFromString(sk)
	if err != nil {
		return err
	}
	tx, err := ycc.CreateWithdrawTx(priv, amount, height)
	if err != nil {
		return err
	}
	println(tx)
	return xclient.Call(context.Background(), "SendTx", tx, nil)
}

func withdraw_all(sk string, height int64) error {
	priv, err := crypto.PrivateKeyFromString(sk)
	if err != nil {
		return err
	}
	d, err := query_deposit(crypto.PubkeyToAddr(priv.PublicKey()))
	if err != nil {
		return err
	}
	return withdraw(sk, d, height)
}

func query_balance(addr string) (int64, error) {
	var b int64
	err := xclient.Call(context.Background(), "QueryBalance", addr, &b)
	if err != nil {
		return 0, err
	}
	fmt.Printf("%s balance: %d", addr, b)
	return b, nil
}

func query_deposit(addr string) (int64, error) {
	var b int64
	err := xclient.Call(context.Background(), "QueryDeposit", addr, &b)
	if err != nil {
		return 0, err
	}
	fmt.Printf("%s deposit: %d", addr, b)
	return b, nil
}

func query_tx(hash string) error {
	h, err := types.HashFromString(hash)
	if err != nil {
		return err
	}
	if len(h) != 32 {
		return errors.New("bad hash len")
	}
	tx := new(types.Tx)
	err = xclient.Call(context.Background(), "QueryTx", h, tx)
	if err != nil {
		return err
	}
	println(types.JsonString(tx))
	return nil
}

func lastBlock() error {
	b := new(types.Block)
	err := xclient.Call(context.Background(), "LastBlock", nil, b)
	if err != nil {
		return err
	}
	println(types.JsonString(b))
	return nil
}

func query_block(hash string, height int64) error {
	if hash != "" {
		h, err := types.HashFromString(hash)
		if err != nil {
			return err
		}
		if len(h) != 32 {
			return errors.New("bad hash len")
		}
		b := new(types.Block)
		err = xclient.Call(context.Background(), "QueryBlock", h, b)
		if err != nil {
			return err
		}
		println(types.JsonString(b))
		return nil
	}
	if height < 0 {
		return errors.New("height must > 0")
	}
	b := new(types.Block)
	err := xclient.Call(context.Background(), "QueryBlockByHeight", height, b)
	if err != nil {
		return err
	}
	println(types.JsonString(b))
	return nil
}

func randAddress() string {
	const S = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	l := len(S)

	s := make([]byte, 10)
	for i := 0; i < 10; i++ {
		r := rand.Intn(l)
		s[i] = S[r]
	}
	return crypto.NewAddress(crypto.Hash(s))
}

func runSendTx() error {
	skSeed := "4f9db771073ee5c51498be842c1a9428edbc992a91e0bac65585f39a642d3a05"
	sk, err := crypto.PrivateKeyFromString(skSeed)
	if err != nil {
		return err
	}

	ch := make(chan *types.Tx, 1024)
	go func() {
		for {
			to := randAddress()
			tx, err := coin.CreateTransferTx(sk, to, 1, 0)
			if err != nil {
				log.Println(err)
				return
			}
			ch <- tx
		}
	}()

	const N = 256
	txs := make([]*types.Tx, N)
	i := 0
	for tx := range ch {
		txs[i] = tx
		i++
		if i == N {
			i = 0
			xclient.Call(context.Background(), "SendTxs", txs, &struct{}{})
			// time.Sleep(time.Second)
		}
	}
	return nil
}

type task struct {
	pk     crypto.PublicKey
	m, sig []byte
	ch     chan bool
}

func (t *task) Do() {
	t.ch <- crypto.Verify(t.pk, t.m, t.sig)
}

func test2(count int) {
	ts := make([]*task, count)
	m := []byte("asdfalsdfalsdfladfalsdfalsdfladfjaldfajlsdf")
	sk, _ := crypto.NewKey()
	pk := sk.PublicKey()
	ch := make(chan bool, count)
	for i := 0; i < count; i++ {
		ts[i] = &task{pk: pk, m: m, sig: crypto.Sign(sk, m), ch: ch}
	}

	bt := time.Now()
	for _, t := range ts {
		t.Do()
	}
	fmt.Println("verify", count, "用时", time.Since(bt))
}

func test1(count int) {
	pool := utils.NewPool(8, 100)
	ts := make([]*task, count)
	m := []byte("asdfalsdfalsdfladfalsdfalsdfladfjaldfajlsdf")
	sk, _ := crypto.NewKey()
	pk := sk.PublicKey()
	ch := make(chan bool, count)
	for i := 0; i < count; i++ {
		ts[i] = &task{pk: pk, m: m, sig: crypto.Sign(sk, m), ch: ch}
	}
	go pool.Run()

	bt := time.Now()
	go func() {
		for _, t := range ts {
			pool.Put(t)
		}
	}()
	k := 0
	for range ch {
		k++
		if k == count {
			break
		}
	}
	close(ch)
	fmt.Println("verify", count, "用时", time.Since(bt))
}
