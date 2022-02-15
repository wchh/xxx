// xxx chain
//lint:file-ignore U1000 Ignore all unused code

package main

import (
	"bytes"
	"flag"
	"fmt"
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

var mlog = log.New("main")

func main() {
	flag.Parse()

	conf := config.DefaultConfig
	_, err := toml.DecodeFile(*confPath, conf)
	if err != nil {
		buf := new(bytes.Buffer)
		encoder := toml.NewEncoder(buf)
		err = encoder.Encode(conf)
		if err != nil {
			panic(err)
		}
		fmt.Println(buf.String())
		os.WriteFile(*confPath, buf.Bytes(), 0666)
		return
	}

	log.Init(conf.LogPath, conf.LogLevel)
	defer log.Sync()

	mlog.Info("xxx start run!!!")

	if conf.NodeType == "consensus" {
		runConsensusNode(conf)
	} else if conf.NodeType == "data" {
		runDataNode(conf)
	} else {
		panic("not support")
	}
}

func runConsensusNode(conf *Config) {
	cc := contract.New(conf.Contract)
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
