package config

type Config struct {
	ChainID     string
	Version     string
	NodeType    string
	PrivateSeed string
	DataPath    string
	FundAddr    string
	LogPath     string
	LogLevel    string
	ServerPort  int
	RpcPort     int

	*ConsensusConfig
	*ChainConfig
}

type ConsensusConfig struct {
	DataNodes     []string
	BootPeers     []string
	GenesisAddr   string
	VotePrice     int64
	TxFee         int64
	AdvSortBlocks int
	AdvVoteBlocks int
	CheckSig      bool
	Single        bool
}

type ChainConfig struct {
}
