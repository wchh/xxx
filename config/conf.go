package config

type Config struct {
	ChainID    string
	Version    string
	NodeType   string
	DataPath   string
	LogPath    string
	LogLevel   string
	ServerPort int
	RpcPort    int

	Consensus *ConsensusConfig
	Chain     *ChainConfig
	Contract  *ConstractConfig
}

type ConsensusConfig struct {
	DataNode             string
	BootPeers            []string
	PrivateSeed          string
	GenesisSeed          string
	GenesisIssueAmount   int64
	GenesisDepositAmount int64
	VotePrice            int64
	AdvSortBlocks        int
	AdvVoteBlocks        int
	CheckSig             bool
	Single               bool
}

type ChainConfig struct {
	ShardNum  int
	PreBlocks int // 提前
}

var DefaultConfig = &Config{
	ChainID:    "xxx",
	Version:    "0.0.1",
	NodeType:   "consensus",
	DataPath:   "data",
	LogPath:    "log.log",
	LogLevel:   "debug",
	ServerPort: 12123,
	RpcPort:    12223,

	Consensus: &ConsensusConfig{
		PrivateSeed:          "4f9db771073ee5c51498be842c1a9428edbc992a91e0bac65585f39a642d3a05",
		GenesisSeed:          "4f9db771073ee5c51498be842c1a9428edbc992a91e0bac65585f39a642d3a05",
		GenesisIssueAmount:   1e8 * 100,
		GenesisDepositAmount: 1000,
		VotePrice:            1000,
		AdvSortBlocks:        10,
		AdvVoteBlocks:        5,
		CheckSig:             true,
		Single:               true,
	},
	Chain: &ChainConfig{
		ShardNum:  4,
		PreBlocks: 4,
	},
	Contract: &ConstractConfig{
		FundAddr:    "3ftLmbf3MEPXTJYmd4HsRtKqUgy",
		GenesisAddr: "3VB31vq79eWwikFTFZknfYm9jJgC",
		TxFee:       1000,
	},
}

type ConstractConfig struct {
	FundAddr    string
	GenesisAddr string
	TxFee       int64
}

// genesis key
/*******************************************
sk: 4f9db771073ee5c51498be842c1a9428edbc992a91e0bac65585f39a642d3a05
pk: b26c75988f8d01b829a8cb0cd15174a418999e953850fcf797c2594c782ec444
address: 3VB31vq79eWwikFTFZknfYm9jJgC
*******************************************/

// miner
/*******************************************
sk: 0080242bfc85666aa8ce21846fa78d24898509fa8a60dd47ae80556798739617
pk: 8816caf90abe305954f555403c138511b372689f009e98f6cad38f8000053d41
address: 2txyQ8pGDRfHHeoPikeJ2tcemiVt
*******************************************/

// fund key
/*******************************************
sk: abb989c841312835d6ae22f68bbec4734562d110e081ac48d8ad042b7783ecb3
pk: 034e09f7602359487e5678e770dd7d263af68e5ac2c8b7b1877271c88922af87
address: 3ftLmbf3MEPXTJYmd4HsRtKqUgy
*******************************************/
