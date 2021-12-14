package chain

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"xxx/config"
	"xxx/contract/coin"
	"xxx/crypto"
	"xxx/log"
	"xxx/types"

	"github.com/smallnest/rpcx/client"
)

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

func runSendTx(xclient client.XClient) error {
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
				fmt.Println(err)
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
			err := xclient.Call(context.Background(), "SendTxs", txs, nil)
			if err != nil {
				panic(err)
			}
			i = 0
		}
	}
	return nil
}

func runServer(conf *config.Config) {
	conf.NodeType = "Chain"
	chain, err := New(conf)
	if err != nil {
		panic(err)
	}
	chain.Run()
}

func TestSendTxs(t *testing.T) {
	conf := config.DefaultConfig
	log.Init(conf.LogPath, conf.LogLevel)
	defer log.Sync()

	go runServer(conf)
	time.Sleep(time.Second * 1)
	d, err := client.NewPeer2PeerDiscovery("tcp@localhost:"+fmt.Sprintf("%d", conf.RpcPort), "")
	if err != nil {
		t.Fatal(err)
	}
	opt := client.DefaultOption
	xclient := client.NewXClient("Chain", client.Failtry, client.RandomSelect, d, opt)
	go runSendTx(xclient)
	time.Sleep(time.Second * 10)
}

func TestGetPreBlock(t *testing.T) {

}

func TestGetBlock(t *testing.T) {

}

func TestQueryBlock(t *testing.T) {

}
