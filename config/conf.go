package config

type ConsensusConfig struct {
	ChainID  string
	Version  string
	DataPath string
	LogPath  string
	LogLevel string

	BootPeers        []string
	DataNodeRpc      string
	DataNodePID      string
	ServerPort       int
	RpcPort          int
	UseRpcToDataNode bool

	PrivateSeed string
	GenesisSeed string
	FundAddress string

	GenesisIssueAmount   int64
	GenesisDepositAmount int64
	CoinPricition        int64

	VotePrice   float64
	BlockReward float64
	VoteReward  float64
	MakerReward float64
	TxFee       float64

	AdvSortBlocks int
	AdvVoteBlocks int

	CheckSig bool
	Single   bool
}

type DataNodeConfig struct {
	ChainID     string
	Version     string
	DataPath    string
	LogPath     string
	LogLevel    string
	PrivateSeed string

	RpcPort     int
	ServerPort  int
	PreBlocks   int // 提前
	MaxBlockTxs int
}

var DefaultConsensusConfig = &ConsensusConfig{
	ChainID:              "xxx",
	Version:              "0.0.1",
	DataPath:             "./data/",
	LogPath:              "./logs/",
	LogLevel:             "debug",
	RpcPort:              10801,
	ServerPort:           10901,
	DataNodeRpc:          "localhost:10801",
	DataNodePID:          "/ip4/192.168.0.143/tcp/10901/p2p/12D3KooWMprfHvNnmXqLZtUfEdKS9FZocZNz8ir7Vax43nb4af1M",
	BootPeers:            []string{},
	FundAddress:          "3ftLmbf3MEPXTJYmd4HsRtKqUgy",
	PrivateSeed:          "4f9db771073ee5c51498be842c1a9428edbc992a91e0bac65585f39a642d3a05",
	GenesisSeed:          "4f9db771073ee5c51498be842c1a9428edbc992a91e0bac65585f39a642d3a05",
	GenesisIssueAmount:   1e8 * 100,
	GenesisDepositAmount: 1000,
	CoinPricition:        1e8,
	VotePrice:            1000,
	BlockReward:          10,
	VoteReward:           0.2,
	MakerReward:          0.1,
	TxFee:                0.001,
	AdvSortBlocks:        10,
	AdvVoteBlocks:        5,
	CheckSig:             true,
	Single:               true,
}

var DefaultDataNodeConfig = &DataNodeConfig{
	ChainID:     "xxx",
	Version:     "0.0.1",
	DataPath:    "./data/",
	LogPath:     "./logs/",
	LogLevel:    "debug",
	PrivateSeed: "0080242bfc85666aa8ce21846fa78d24898509fa8a60dd47ae80556798739617",
	RpcPort:     10801,
	ServerPort:  10901,
	PreBlocks:   4,
}

// genesis key
/*******************************************
sk: 4f9db771073ee5c51498be842c1a9428edbc992a91e0bac65585f39a642d3a05
pk: b26c75988f8d01b829a8cb0cd15174a418999e953850fcf797c2594c782ec444
address: 3VB31vq79eWwikFTFZknfYm9jJgC
*******************************************/

// datanode priv for p2p
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
