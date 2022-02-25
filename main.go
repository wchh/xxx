// xxx chain
//lint:file-ignore U1000 Ignore all unused code

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"time"

	"xxx/chain"
	"xxx/config"
	"xxx/consensus"
	"xxx/log"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"
)

var mlog = log.New("main")

func main() {
	flag.Parse()

	logCh := make(chan logInfo)
	defer close(logCh)

	go func() {
		li := <-logCh
		log.Init(li.path, li.level)
		mlog.Info("xxx start run!!!")
	}()
	defer log.Sync()

	app := &cli.App{
		Commands: []*cli.Command{
			consensusCmd(logCh),
			datanodeCmd(logCh),
		},
	}
	app.Run(os.Args)
}

type logInfo struct {
	path  string
	level string
}

func readConfig(path string, conf interface{}) {
	_, err := toml.DecodeFile(path, conf)
	if err != nil {
		buf := new(bytes.Buffer)
		encoder := toml.NewEncoder(buf)
		err = encoder.Encode(conf)
		if err != nil {
			panic(err)
		}
		fmt.Println(buf.String())
		os.WriteFile(path, buf.Bytes(), 0666)
	}
}

func datanodeCmd(logCh chan<- logInfo) *cli.Command {
	return &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "config file path",
				Value:   "data.toml",
			},
		},
		Name:    "data",
		Aliases: []string{"dt"},
		Usage:   "run data node",
		Action: func(c *cli.Context) error {
			conf := config.DefaultDataNodeConfig
			readConfig(c.String("config"), conf)
			logCh <- logInfo{conf.LogPath, conf.LogLevel}
			time.Sleep(time.Millisecond)
			runDataNode(conf)
			return nil
		},
	}
}

func consensusCmd(logCh chan<- logInfo) *cli.Command {
	return &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "config file path",
				Value:   "xxx.toml",
			},
		},
		Name:    "consensus",
		Aliases: []string{"cons"},
		Usage:   "run consensus node",
		Action: func(c *cli.Context) error {
			conf := config.DefaultConsensusConfig
			readConfig(c.String("config"), conf)
			logCh <- logInfo{conf.LogPath, conf.LogLevel}
			time.Sleep(time.Millisecond)
			runConsensusNode(conf)
			return nil
		},
	}
}

func runConsensusNode(conf *config.ConsensusConfig) {
	consensus, err := consensus.New(conf)
	if err != nil {
		panic(err)
	}
	consensus.Run()
}

func runDataNode(conf *config.DataNodeConfig) {
	ch, err := chain.New(conf)
	if err != nil {
		panic(err)
	}
	ch.Run()
}
