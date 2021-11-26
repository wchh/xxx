// xxx chain
//lint:file-ignore U1000 Ignore all unused code

package xxx

import (
	"flag"

	"xxx/chain"
	"xxx/consensus"
	"xxx/contract"
	"xxx/contract/coin"
	"xxx/contract/ycc"
	"xxx/log"
	"xxx/p2p"

	"github.com/BurntSushi/toml"
)

var confPath = flag.String("c", "xxx.toml", "config path")

func main() {
	flag.Parse()

	var conf config
	_, err := toml.DecodeFile(*confPath, &conf)
	if err != nil {
		panic(err)
	}
	log.Init(conf.LogPath, conf.LogLevel)
	defer log.Sync()
	log.New("main").Info("xxx start run!!!")
	if conf.Node == "data" {
		runConsensusNode(&conf)
	} else {
		runDataNode(&conf)
	}
}

type config struct {
	Consensus *consensus.Conf
	P2P       *p2p.Conf
	Chain     *chain.Conf
	Node      string // consensus, data or normal
	FundAddr  string
	LogPath   string
	LogLevel  string
}

func runConsensusNode(conf *config) {
	cc := contract.New(conf.FundAddr)
	coin.Init(cc)
	ycc.Init(cc)

	node, err := p2p.NewNode(conf.P2P)
	if err != nil {
		panic(err)
	}

	consensus, err := consensus.New(conf.Consensus, node, cc)
	if err != nil {
		panic(err)
	}
	consensus.Run()
}

func runDataNode(conf *config) {
	node, err := p2p.NewNode(conf.P2P)
	if err != nil {
		panic(err)
	}

	ch, err := chain.New(conf.Chain, node)
	if err != nil {
		panic(err)
	}
	ch.Run()
}
