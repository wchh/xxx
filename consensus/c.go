package consensus

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"time"

	"xxx/config"
	"xxx/contract"
	"xxx/contract/coin"
	"xxx/contract/ycc"
	"xxx/crypto"
	"xxx/db"
	"xxx/log"
	"xxx/p2p"
	"xxx/types"
	"xxx/utils"

	"github.com/smallnest/rpcx/client"
)

var clog = log.New("consensus")

type Consensus struct {
	*config.ConsensusConfig
	db     db.DB
	node   *p2p.Node
	cc     *contract.Container
	rpcClt client.XClient

	chid      string
	version   string
	rpcPort   int
	lastBlock *types.Block
	priv      crypto.PrivateKey
	myAddr    string

	maker_mp      map[int64]map[int]*maker
	committee_mp  map[int64]map[int]*committee
	preblock_mp   map[int64]*types.Block
	alldeposit_mp map[int64]int
	block_mp      map[int64]*types.Block
	peer_mp       map[string]*types.PeerInfo

	nbch chan *types.Block
	mch  chan *types.PMsg

	pool *utils.GoPool
}

func initContracts(conf *config.ConsensusConfig) *contract.Container {
	cc := contract.New(conf)
	ycc.Init(cc)
	coin.Init(cc)
	return cc
}

func New(conf *config.ConsensusConfig) (*Consensus, error) {
	ldb, err := db.NewLDB(conf.DataPath)
	if err != nil {
		return nil, err
	}
	if conf.VotePrice == 0 {
		conf.VotePrice = 1000
	}
	priv, err := crypto.PrivateKeyFromString(conf.PrivateSeed)
	if err != nil {
		return nil, err
	}
	p2pConf := &p2p.Conf{Priv: priv, Port: conf.ServerPort, Topics: types.ConsensusTopcs}
	node, err := p2p.NewNode(p2pConf)
	if err != nil {
		return nil, err
	}
	cc := initContracts(conf)
	return &Consensus{
		chid:            conf.ChainID,
		version:         conf.Version,
		rpcPort:         conf.RpcPort,
		priv:            priv,
		ConsensusConfig: conf,
		node:            node,
		db:              ldb,
		cc:              cc,
		myAddr:          crypto.PubkeyToAddr(priv.PublicKey()),
		maker_mp:        make(map[int64]map[int]*maker),
		committee_mp:    make(map[int64]map[int]*committee),
		preblock_mp:     make(map[int64]*types.Block),
		alldeposit_mp:   make(map[int64]int),
		block_mp:        make(map[int64]*types.Block),
		peer_mp:         make(map[string]*types.PeerInfo),

		nbch: make(chan *types.Block, 1),
		mch:  make(chan *types.PMsg, 256),

		pool: utils.NewPool(8),
	}, nil
}

func (c *Consensus) clean(height int64) {
	N := int64(c.AdvSortBlocks + 10)
	for h := range c.maker_mp {
		if h < height-N {
			delete(c.maker_mp, h)
		}
	}
	for h := range c.committee_mp {
		if h < height-N {
			delete(c.committee_mp, h)
		}
	}
	for h := range c.preblock_mp {
		if h < height-N {
			delete(c.preblock_mp, h)
		}
	}
	for h := range c.alldeposit_mp {
		if h < height-N {
			delete(c.alldeposit_mp, h)
		}
	}
	for h := range c.block_mp {
		if h < height-N {
			delete(c.block_mp, h)
		}
	}
}

func (c *Consensus) genesisBlock() (*types.Block, error) {
	gsk, err := crypto.PrivateKeyFromString(c.GenesisSeed)
	if err != nil {
		clog.Errorw("GenesisSeed error", "err", err)
		return nil, err
	}

	ia := coin.CoinX * c.GenesisIssueAmount
	gaddr := gsk.PublicKey().Address()
	tx0, err := coin.CreateIssueTx(gsk, gaddr, ia)
	if err != nil {
		return nil, err
	}
	da := c.GenesisDepositAmount
	tx1, err := ycc.CreateDepositTx(gsk, gaddr, da, 0)
	if err != nil {
		return nil, err
	}
	txs := []*types.Tx{tx0, tx1}
	merkel := types.TxsMerkel(txs)

	var nilHash [32]byte

	h := &types.Header{
		ParentHash: nilHash[:],
		Height:     0,
		Round:      0,
		TxsHash:    merkel,
	}

	return &types.Block{
		Header: h,
		Txs:    txs,
	}, nil
}

func (c *Consensus) handlePMsg(msg *types.PMsg) {
	switch msg.Topic {
	case types.BlockTopic:
		c.handleBlocks(msg.Msg.(*types.BlocksReply))
	case types.PreBlockTopic:
		c.handlePreBlock(msg.Msg.(*types.PreBlock))
	case types.MakerSortTopic:
		c.handleMakerSort(msg.Msg.(*types.Sortition))
	case types.CommitteeSortTopic:
		c.handleCommitteeSort(msg.Msg.(*types.Sortition))
	case types.MakerVoteTopic:
		c.handleMakerVote(msg.Msg.(*types.Vote))
	case types.CommitteeVoteTopic:
		c.handleCommitteeVote(msg.Msg.(*types.CommitteeVote))
	case types.ConsensusBlockTopic:
		c.handleConsensusBlock(msg.Msg.(*types.NewBlock))
	}
}

func (c *Consensus) readP2pMsg() {
	for msg := range c.node.C {
		pm, err := c.unmashalMsg(msg)
		if err != nil {
			clog.Errorw("decdoeP2pMsg error", "err", err)
			continue
		}
		c.mch <- pm
	}
}

func (c *Consensus) unmashalMsg(msg *types.GMsg) (*types.PMsg, error) {
	var umsg types.Message
	switch msg.Topic {
	case types.PreBlockTopic:
		var m types.PreBlock
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case types.MakerSortTopic:
		var m types.Sortition
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case types.CommitteeSortTopic:
		var m types.Sortition
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case types.MakerVoteTopic:
		var m types.CommitteeVote
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case types.CommitteeVoteTopic:
		var m types.CommitteeVote
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case types.ConsensusBlockTopic:
		var m types.NewBlock
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	}
	return &types.PMsg{PID: msg.PID, Topic: msg.Topic, Msg: umsg}, nil
}

func (c *Consensus) Run() {
	c.newRpcClient()
	go runRpc(fmt.Sprintf(":%d", c.rpcPort), c)
	c.sync()
	go c.readP2pMsg()
	c.pool.Run()
	c.ConsensusRun()
}

func (c *Consensus) handleNewblock(b *types.Block, round int) {
	clog.Infow("handleNewblock", "height", b.Header.Height)
	c.makeBlock(b, round)
	c.sortition(b, round)
	c.voteMaker(b.Header.Height+int64(c.AdvVoteBlocks), round)
	c.voteCommittee(b.Header.Height+int64(c.AdvVoteBlocks), round)
	c.clean(b.Header.Height)
	go c.getPreBlocks(b.Header.Height + 2)
}

func (c *Consensus) ConsensusRun() {
	round := 0
	blockTimeout := time.Second * 4
	tm1Ch := make(chan int64, 1)
	tm2Ch := make(chan int64, 1)
	tm3Ch := make(chan int64, 1)

	for {
		select {
		case msg := <-c.mch:
			c.handlePMsg(msg)
		case b := <-c.nbch:
			round = 0
			c.handleNewblock(b, round)
			time.AfterFunc(blockTimeout, func() { tm1Ch <- b.Header.Height })
		case height := <-tm3Ch:
			if c.lastBlock.Header.Height == height {
				c.makeBlock(c.lastBlock, round)
			}
		case height := <-tm2Ch:
			if c.lastBlock.Header.Height == height {
				c.voteMaker(height, round)
				c.voteCommittee(height, round)
				time.AfterFunc(time.Second, func() { tm2Ch <- height })
			}
		case height := <-tm1Ch:
			if c.lastBlock.Header.Height == height {
				round++
				c.sortition(c.lastBlock, round)
				time.AfterFunc(blockTimeout, func() { tm1Ch <- height })
				time.AfterFunc(time.Second, func() { tm2Ch <- height + 1 })
			}
		}
	}
}

func (c *Consensus) makeBlock(pb *types.Block, round int) {
	height := pb.Header.Height + 1
	c.getCommittee(height, round).setCommittee()

	clog.Infow("makeBlock", "height", height, "round", round)

	maker := c.getmaker(height, round)
	maker.setMaker()
	if maker.my == nil {
		clog.Infow("makeBlock Not sort")
		return
	}
	vs, ok := maker.mvmp[string(maker.my.Hashs[0].Hash)]
	if !ok {
		clog.Infow("makeBlock Not vote maker")
		return
	}
	if len(vs) < MustVotes {
		clog.Infow("makeBlock Not enough votes")
		return
	}

	m := &types.Ycc_Mine{
		Votes: vs,
		Sort:  maker.my,
	}
	data, err := types.Marshal(m)
	if err != nil {
		panic(err)
	}
	tx0 := &types.Tx{
		Contract: ycc.Name,
		Op:       ycc.MineOp,
		Data:     data,
	}

	tx0.Sign(c.priv)
	b, ok := c.preblock_mp[height]
	if !ok {
		clog.Infow("makeBlock preBlock not in cache", "height", height)
		b = &types.Block{
			Header: &types.Header{
				Height: height,
				// Round:      int32(round),
				BlockTime:  time.Now().Unix(),
				ParentHash: pb.Hash(),
			},
		}
		// return
	}
	b.Header.Round = int32(round)
	b.Txs = append([]*types.Tx{tx0}, b.Txs...)

	nb, err := c.perExecBlock(b)
	if err != nil {
		panic(err)
	}

	c.node.Publish(types.ConsensusBlockTopic, nb)
	c.handleConsensusBlock(nb)
}

func (c *Consensus) handlePreBlock(b *types.PreBlock) {
	txs := b.B.Txs
	if c.CheckSig {
		txs, _, _ = c.txsVerifySig(txs, 8, false)
		b.B.Txs = txs
	}
	c.preblock_mp[b.B.Header.Height] = b.B
}

func (c *Consensus) handleMakerVote(v *types.Vote) {
	maker := c.getmaker(v.Height, int(v.Round))
	maker.mvmp[string(v.Hash)] = append(maker.mvmp[string(v.Hash)], v)
}

func (c *Consensus) handleCommitteeVote(v *types.CommitteeVote) {
	comm := c.getCommittee(v.Height, int(v.Round))

	for _, h := range v.CommHashs {
		if !comm.findSort(string(h)) {
			return
		}
		comm.svmp[string(h)] += len(v.MyHashs)
	}
}

const lastHeaderKey = "lastheader"

func writeLastHeader(h *types.Header, db db.KV) error {
	data, err := types.Marshal(h)
	if err != nil {
		return err
	}
	return db.Set([]byte(lastHeaderKey), data)
}

func readLastHeader(db db.KV) (*types.Header, error) {
	val, err := db.Get([]byte(lastHeaderKey))
	if err != nil {
		return nil, err
	}
	h := new(types.Header)
	err = types.Unmarshal(val, h)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func (c *Consensus) perExecBlock(b *types.Block) (*types.NewBlock, error) {
	var failedHash [][]byte

	clog.Infow("preExecBlock", "height", b.Header.Height, "round", b.Header.Round, "ntx", len(b.Txs))
	txs := b.Txs
	if c.CheckSig {
		txs, failedHash, _ = c.txsVerifySig(txs, 8, false)
	}

	db := db.NewMDB(c.db)
	c.cc.SetDB(db)

	rtxs := make([]*types.Tx, len(txs))
	index := 0
	for _, tx := range txs {
		err := c.cc.ExecTx(tx)
		if err != nil {
			failedHash = append(failedHash, tx.Hash())
		} else {
			rtxs[index] = tx
		}
		index++
	}
	rtxs = rtxs[:index]
	b.Header.TxsHash = types.TxsMerkel(rtxs)
	b.Txs = rtxs

	writeLastHeader(b.Header, db)

	comm := c.getCommittee(b.Header.Height, int(b.Header.Round))
	comm.db = db
	comm.b = b
	return &types.NewBlock{Header: b.Header, FailedHashs: failedHash, Tx0: b.Txs[0]}, nil
}

func doErr(err *error, f func()) {
	if err != nil {
		f()
	}
}

func (c *Consensus) execBlock(b *types.Block) error {
	clog.Infow("execBlock", "height", b.Header.Height, "round", b.Header.Round, "ntx", len(b.Txs))
	t, err := c.db.OpenTransaction()
	if err != nil {
		return err
	}
	// kv := db.NewBKV(t)
	c.cc.SetDB(t)

	defer doErr(&err, func() {
		t.Discard()
	})

	txs := b.Txs
	if c.CheckSig {
		txs, _, err = c.txsVerifySig(txs, 8, true)
		if err != nil {
			return err
		}
	}

	// TODO: parallel exec txs
	for _, tx := range txs {
		err = c.cc.ExecTx(tx)
		if err != nil {
			clog.Errorw("execBlock exectx error", "err", err)
			return err
		}
	}

	err = writeLastHeader(b.Header, t)
	if err != nil {
		return err
	}

	return t.Commit()
}

func (c *Consensus) setBlock(nb *types.NewBlock) {
	height := nb.Header.Height
	round := int(nb.Header.Round)
	clog.Infow("setBlock", "height", height, "round", round, "hash", types.Hash(nb.Header.Hash()))
	comm := c.getCommittee(height, round)

	b := comm.b
	if b != nil && bytes.Equal(nb.Header.TxsHash, b.Header.TxsHash) { // my made block is ok
		comm.db.Commit()
	} else {
		pb := c.preblock_mp[height]
		txs := []*types.Tx{nb.Tx0}
		for _, tx := range pb.Txs {
			for _, fh := range nb.FailedHashs {
				if string(tx.Hash()) != string(fh) {
					txs = append(txs, tx)
				}
			}
		}

		if string(nb.Header.TxsHash) != string(types.TxsMerkel(txs)) {
			panic("merkel NOT same")
		}

		b = &types.Block{Header: nb.Header, Txs: txs}
		err := c.execBlock(b)
		if err != nil {
			panic(err)
		}
		comm.b = b
	}

	c.addNewBlock(b)
	c.node.Publish(types.NewBlockTopic, nb)
}

func (c *Consensus) addNewBlock(b *types.Block) {
	c.lastBlock = b // b is complete block
	c.block_mp[b.Header.Height] = b
	clog.Infow("addNewblock", "height", b.Header.Height)
	go func() {
		c.nbch <- b
	}()
}

func (c *Consensus) handleConsensusBlock(b *types.NewBlock) {
	c.setBlock(b)
}

func (c *Consensus) difficulty(t, r int, h int64) float64 {
	all, ok := c.alldeposit_mp[h]
	if !ok {
		n, err := ycc.QueryAllDeposit(c.db)
		if err != nil {
			clog.Errorw("diff error", "err", err)
			return 1
		}
		// all = int(n / c.VotePrice)
		c.alldeposit_mp[h] = int(n)
	}

	allf := float64(all) * math.Pow(0.9, float64(r))
	diff := float64(t) / allf
	return diff
}

func (c *Consensus) verifySort(s *types.Sortition, n int) error {
	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	sortHeight := height - int64(c.AdvSortBlocks)
	if sortHeight < 0 {
		sortHeight = 0
	}
	b, ok := c.block_mp[sortHeight]
	if !ok {
		return fmt.Errorf("verifySort error: NO block in the cache. height=%d", height)
	}
	diff := c.difficulty(n, round, height)
	clog.Infow("verifySort", "height", height, "round", round, "diff", diff, "seed", utils.Bytes(b.Hash()))
	in := &types.VrfInput{Height: height, Round: int32(round), Seed: b.Hash()}
	if err := vrfVerify(in, s, diff); err != nil {
		return err
	}
	return nil
}

func (c *Consensus) handleMakerSort(s *types.Sortition) {
	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	clog.Infow("handleMakerSort", "height", height, "round", round)
	err := c.verifySort(s, MakerSize)
	if err != nil {
		clog.Errorw("handleMakerSort error:", "err", err)
	}

	comm := c.getCommittee(height, round)
	comm.mss[string(s.Sig.PublicKey)] = s
}

func (c *Consensus) handleCommitteeSort(s *types.Sortition) {
	err := c.verifySort(s, CommitteeSize)
	if err != nil {
		clog.Errorw("handleCommitteeSort error:", "err", err)
	}

	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	comm := c.getCommittee(height, round)
	comm.css[string(s.Sig.PublicKey)] = s

	// check sync
	if len(comm.css) > MustVotes {
		if c.lastBlock.Header.Height+int64(c.AdvSortBlocks) < height {
			go c.getBlocks(c.lastBlock.Header.Height+1, 10)
		}
	}
	clog.Infow("handleCommitteeSort", "height", height, "round", round)
}

func (c *Consensus) sortition(b *types.Block, round int) {
	amount, err := ycc.QueryDeposit(c.myAddr, c.db)
	if err != nil {
		clog.Errorw("sortition error", "err", err)
		return
	}
	height := b.Header.Height + int64(c.AdvSortBlocks)
	seed := b.Hash()
	c.sortMaker(height, round, int(amount), seed)
	c.sortCommittee(height, round, int(amount), seed)
}

func (c *Consensus) sortMaker(height int64, round, count int, seed []byte) {
	input := &types.VrfInput{
		Height: height,
		Round:  int32(round),
		Seed:   seed,
	}

	clog.Infow("sortMaker", "height", height, "round", round, "n", count, "seed", utils.Bytes(seed))
	s, err := c.vrfSortiton(input, count, 1, c.difficulty(MakerSize, round, height))
	if err != nil {
		clog.Errorw("sortMaker vrfSortition error", "err", err, "height", height, "round", round)
		return
	}
	if len(s.Hashs) == 0 {
		clog.Infow("sortMaker hash==nil", "height", height, "round", round)
		return
	}

	// only 1 hash
	sort.Sort(types.SortHashs(s.Hashs))
	s.Hashs = s.Hashs[:1]
	s.Sign(c.priv)
	c.handleMakerSort(s)

	maker := c.getmaker(height, round)
	maker.my = s

	c.node.Publish(types.MakerSortTopic, s)
}

func (c *Consensus) sortCommittee(height int64, round, count int, seed []byte) {
	input := &types.VrfInput{
		Height: height,
		Round:  int32(round),
		Seed:   seed,
	}

	diff := c.difficulty(CommitteeSize, round, height)
	s, err := c.vrfSortiton(input, count, 3, diff)
	if err != nil {
		clog.Errorw("sortCommittee vrfSortition error", "err", err, "height", height, "round", round)
		return
	}
	if len(s.Hashs) == 0 {
		clog.Infow("sortCommittee hash==nil", "height", height, "round", round)
		return
	}

	clog.Infow("sortCommittee", "height", height, "round", round, "count", count, "diff", diff, "nhash", len(s.Hashs))

	s.Sign(c.priv)
	c.handleCommitteeSort(s)
	c.getCommittee(height, round).myss = s
	c.node.Publish(types.CommitteeSortTopic, s)
}

func (c *Consensus) voteMaker(height int64, round int) {
	comm := c.getCommittee(height, round)
	myHashs, _ := comm.getMyHashs()
	if myHashs == nil {
		return
	}
	mhs := comm.getMakerHashs()
	if len(mhs) == 0 {
		return
	}
	v := &types.Vote{
		Height:  height,
		Round:   int32(round),
		Hash:    mhs[0], // vote first
		MyHashs: myHashs,
	}
	v.Sign(c.priv)
	c.node.Publish(types.MakerVoteTopic, v)
	c.handleMakerVote(v)
}

func (c *Consensus) voteCommittee(height int64, round int) {
	comm := c.getCommittee(height, round)
	myHashs, comHashs := comm.getMyHashs()
	if myHashs == nil {
		return
	}
	cmmtt := &types.CommitteeVote{
		Height:    height,
		Round:     int32(round),
		CommHashs: comHashs,
		MyHashs:   myHashs,
	}

	c.node.Publish(types.CommitteeVoteTopic, cmmtt)
	c.handleCommitteeVote(cmmtt)
}

func (c *Consensus) handleBlocks(m *types.BlocksReply) bool {
	for _, b := range m.Bs {
		if b.Header.Height != c.lastBlock.Header.Height+1 {
			clog.Errorw("handleBlocksReply error", "height NOT match", "height", b.Header.Height)
			return false
		}
		if string(b.Hash()) != string(c.lastBlock.Hash()) {
			clog.Errorw("handleBlocksReply error", "parent hash NOT match", "height", b.Header.Height)
			return false
		}
		err := c.execBlock(b)
		if err != nil {
			clog.Errorw("handleBlocksReply execBlock error", "err", err, "height", b.Header.Height)
			return false
		}
		c.lastBlock = b
	}

	synced := m.LastHeight == m.Bs[len(m.Bs)-1].Header.Height
	if synced {
		c.addNewBlock(c.lastBlock)
	}
	return synced
}

func (c *Consensus) sync() {
	lastHeader, err := readLastHeader(c.db)
	if err != nil {
		clog.Errorw("readLastHeader error", "err", err)
		gb, err := c.genesisBlock()
		if err != nil {
			clog.Panicw("GenesisBlock panic", "err", err)
		}
		err = c.execBlock(gb)
		if err != nil {
			clog.Panicw("GenesisBlock exec panic", "err", err)
		}
		c.addNewBlock(gb)
		if c.Single {
			c.firstSort(gb)
			return
		}
	} else {
		c.addNewBlock(&types.Block{Header: lastHeader})
	}
	synced := false
	for !synced {
		br, err := c.rpcGetBlocks(lastHeader.Height, 10, "GetBlocks")
		if err != nil {
			clog.Errorw("sync error", "err", err)
			return
		}
		synced = c.handleBlocks(br)
	}
}

func (c *Consensus) firstSort(zb *types.Block) {
	n, err := ycc.QueryDeposit(c.myAddr, c.db)
	if err != nil {
		clog.Errorw("sortition error", "err", err)
		return
	}

	seed := zb.Hash()
	round := 0
	for i := int64(0); i < int64(c.AdvSortBlocks); i++ {
		c.sortMaker(i, round, int(n), seed)
		c.sortCommittee(i, round, int(n), seed)
		if i < int64(c.AdvVoteBlocks) {
			c.voteMaker(i, round)
			c.voteCommittee(i, round)
		}
	}
}

// TODO: peerlist and data node list
