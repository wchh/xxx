// xxx chain
//lint:file-ignore U1000 Ignore all unused code

package xxx

import (
	"flag"
	"os"

	"xxx/chain"
	"xxx/config"
	"xxx/consensus"
	"xxx/contract"
	"xxx/contract/coin"
	"xxx/contract/ycc"
	"xxx/log"

	"github.com/BurntSushi/toml"
)

var confPath = flag.String("c", "xxx.toml", "config path")

type Config = config.Config

func main() {
	flag.Parse()

	var conf Config
	_, err := toml.DecodeFile(*confPath, &conf)
	if err != nil {
		f, err := os.OpenFile(*confPath, os.O_CREATE|os.O_APPEND, os.ModePerm)
		if err != nil {
			panic(err)
		}
		encoder := toml.NewEncoder(f)
		encoder.Encode(conf)
		return
	}

	log.Init(conf.LogPath, conf.LogLevel)
	defer log.Sync()
	log.New("main").Info("xxx start run!!!")

	if conf.NodeType == "data" {
		runConsensusNode(&conf)
	} else {
		runDataNode(&conf)
	}
}

func runConsensusNode(conf *Config) {
	cc := contract.New(conf.FundAddr)
	coin.Init(cc)
	ycc.Init(cc)

	consensus, err := consensus.New(conf, cc)
	if err != nil {
		panic(err)
	}
	consensus.Run()
}

func runDataNode(conf *Config) {
	ch, err := chain.New(conf)
	if err != nil {
		panic(err)
	}
	ch.Run()
}
