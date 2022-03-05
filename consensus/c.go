package consensus

import (
	"context"
	"errors"
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

	pool    *utils.GoPool
	txsPool *utils.LablePool
}

func initContracts(conf *config.ConsensusConfig) *contract.Container {
	cc := contract.New(conf)
	ycc.Init(cc)
	coin.Init(cc)
	return cc
}

func New(conf *config.ConsensusConfig) (*Consensus, error) {
	// create levalDB
	ldb, err := db.NewLDB(conf.DataPath)
	if err != nil {
		return nil, err
	}

	// votePrice
	if conf.VotePrice == 0 {
		conf.VotePrice = 1000
	}

	// get private key for sign and p2p
	priv, err := crypto.PrivateKeyFromString(conf.PrivateSeed)
	if err != nil {
		return nil, err
	}

	// create p2p node
	p2pConf := &p2p.Conf{
		Priv:      priv,
		Port:      conf.ServerPort,
		Topics:    types.ConsensusTopcs,
		BootPeers: append(conf.BootPeers, conf.DataNodePID),
		Compress:  conf.CompressP2p,
	}
	node, err := p2p.NewNode(p2pConf)
	if err != nil {
		return nil, err
	}

	// get datanode pid
	tinfo, err := p2p.ParseP2PAddress(conf.DataNodePID)
	if err != nil {
		return nil, err
	}
	conf.DataNodePID = string(tinfo.ID)

	// init contracts: ycc, coin
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

		pool:    utils.NewPool(8, 64),
		txsPool: new(utils.LablePool),
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
		BlockTime:  time.Now().UnixMilli(),
	}

	return &types.Block{
		Header: h,
		Txs:    txs,
	}, nil
}

func (c *Consensus) handlePMsg(msg *types.PMsg) {
	switch msg.Topic {
	case types.BlocksTopic:
		c.handleBlocks(msg.Msg.(*types.BlocksReply))
	case types.PreBlockTopic:
		c.handlePreBlock(msg.Msg.(*types.Block))
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
	case types.BlocksTopic:
		var m types.BlocksReply
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case types.PreBlockTopic:
		var m types.BlocksReply
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		if len(m.Bs) == 0 {
			return nil, errors.New("no preblocks")
		}
		umsg = m.Bs[0]
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
	go c.pool.Run()
	go runRpc(fmt.Sprintf(":%d", c.rpcPort), c)
	go c.readP2pMsg()
	c.connectDatanode()
	c.consensusRun()
}

func (c *Consensus) handleTimeout(round int) {
	height := c.lastBlock.Header.Height
	hash := c.lastBlock.Hash()
	c.sortition(hash, height+1, round)
	c.voteMaker(height+1, round)
	c.voteCommittee(height+1, round)
}

func (c *Consensus) handleNewblock(b *types.Block) {
	clog.Infow("handleNewblock", "height", b.Header.Height)
	c.lastBlock = b // b is complete block
	c.block_mp[b.Header.Height] = b

	if b.Header.Height == 0 {
		if c.Single {
			c.firstSort(b)
		}
	}

	round := 0
	hash := b.Hash()
	height := b.Header.Height
	sortHeight := height + int64(c.AdvSortBlocks)
	voteHeight := height + int64(c.AdvVoteBlocks)
	c.sortition(hash, sortHeight, round)
	c.voteMaker(voteHeight, round)
	c.voteCommittee(voteHeight, round)
	c.clean(height)
	c.getPreBlocks(height + 3)
}

func (c *Consensus) consensusRun() {
	round := 0
	blockTimeout := time.Second * 3
	sortTimeout := time.Second * 1
	tm1Ch := make(chan int64, 1)
	tm2Ch := make(chan int64, 1)
	c.sync()

	clog.Infow("consensusRun")

	for {
		select {
		case msg := <-c.mch:
			c.handlePMsg(msg)
		case b := <-c.nbch:
			round = 0
			c.handleNewblock(b)
			time.AfterFunc(blockTimeout, func() { tm1Ch <- b.Header.Height })
			time.AfterFunc(time.Millisecond, func() { tm2Ch <- b.Header.Height + 1 })
		case height := <-tm2Ch:
			c.makeBlock(height, round)
		case height := <-tm1Ch:
			if c.lastBlock.Header.Height == height {
				round++
				clog.Infow("block timeout", "height", height, "round", round)
				c.handleTimeout(round)
				time.AfterFunc(blockTimeout, func() { tm1Ch <- height })
				time.AfterFunc(sortTimeout, func() { tm2Ch <- height + 1 })
			}
		}
	}
}

func nv(vs []*types.Vote) int {
	n := 0
	for _, v := range vs {
		n += len(v.MyHashs)
	}
	return n
}

func (c *Consensus) makeBlock(height int64, round int) {
	c.getCommittee(height, round).setCommittee()

	maker := c.getmaker(height, round)
	maker.setMaker()
	if maker.my == nil {
		clog.Infow("makeBlock Not sort")
		return
	}
	hash := maker.my.Hashs[0].Hash

	vs, ok := maker.mvmp[string(hash)]
	if !ok {
		clog.Infow("makeBlock Not vote maker")
		return
	}
	vn := nv(vs)
	if vn < MustVotes {
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

	ph := c.lastBlock.Hash()
	tx0.Sign(c.priv)
	b, ok := c.preblock_mp[height]
	if !ok {
		clog.Infow("makeBlock preBlock not in cache", "height", height)
		b = &types.Block{
			Header: &types.Header{},
		}
	}
	b.Header.Height = height
	b.Header.ParentHash = ph
	b.Header.BlockTime = time.Now().UnixMilli()
	b.Header.Round = int32(round)
	b.Txs = append([]*types.Tx{tx0}, b.Txs...)
	clog.Infow("makeBlock", "height", height, "round", round, "myhash", utils.ByteString(hash), "nv", vn, "ntx", len(b.Txs))

	nb, err := c.perExecBlock(b)
	if err != nil {
		panic(err)
	}

	c.node.Publish(types.ConsensusBlockTopic, nb)
	c.handleConsensusBlock(nb)
}

func (c *Consensus) handlePreBlock(b *types.Block) {
	clog.Infow("handlePreBlock", "height", b.Header.Height, "ntx", len(b.Txs))
	c.preblock_mp[b.Header.Height] = b
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

type execTxResult struct {
	tx *types.Tx
	ok bool
}

type execTxTask struct {
	tx *types.Tx
	cc *contract.Container
	ch chan *execTxResult
}

func (t *execTxTask) Do() {
	err := t.cc.ExecTx(t.tx)
	// if err != nil {
	// 	clog.Errorw("execTx error", "err", err)
	// }
	t.ch <- &execTxResult{tx: t.tx, ok: err == nil}
}

func (c *Consensus) execTxs(txs []*types.Tx) []*types.Tx {
	ch := make(chan *execTxResult, 16)
	defer close(ch)
	go func() {
		for _, tx := range txs {
			c.txsPool.Put(&execTxTask{tx: tx, cc: c.cc, ch: ch}, int(tx.Sig.PublicKey[0])%16)
		}
	}()
	index := 0
	for i := 0; i < len(txs); i++ {
		tr := <-ch
		if tr.ok {
			txs[index] = tr.tx
			index++
		}
	}
	txs = txs[:index]
	return txs
}

func (c *Consensus) perExecBlock(b *types.Block) (*types.NewBlock, error) {
	var failedHash [][]byte

	clog.Infow("preExecBlock", "height", b.Header.Height, "round", b.Header.Round, "ntx", len(b.Txs))
	txs := b.Txs
	if c.CheckSig && b.Header.Height > 0 {
		txs, failedHash, _ = c.txsVerifySig(txs, false)
	}

	db := db.NewMDB(c.db)
	c.cc.SetDB(db)

	err := c.cc.ExecTx(txs[0])
	if err != nil {
		return nil, err
	}
	if len(txs) > 1 {
		c.execTxs(txs[1:])
	}

	b.Header.TxsHash = types.TxsMerkel(txs)
	b.Txs = txs

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
	if c.CheckSig && b.Header.Height > 0 {
		_, _, err = c.txsVerifySig(txs, true)
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
	clog.Infow("setBlock", "height", height, "round", round, "hash", utils.ByteString(nb.Header.Hash()))
	comm := c.getCommittee(height, round)

	b := comm.b
	if b != nil && string(nb.Header.TxsHash) == string(b.Header.TxsHash) { // my made block is ok
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
	c.setNewBlock(nb)
}

func (c *Consensus) addNewBlock(b *types.Block) {
	t := time.Now().UnixMilli()
	d := 1000 - t + b.Header.BlockTime
	clog.Infow("addNewBlock", "height", b.Header.Height, "duration", d)
	time.AfterFunc(time.Millisecond*time.Duration(d), func() {
		c.nbch <- b
	})
}

func (c *Consensus) checkBlock(b *types.Block) error {
	return nil
}

func (c *Consensus) checkNewBlock(b *types.NewBlock) error {
	return nil
}

func (c *Consensus) handleConsensusBlock(b *types.NewBlock) {
	err := c.checkNewBlock(b)
	if err != nil {
		clog.Errorw("checkNewBlock error", "err", err, "height", b.Header.Height)
	} else {
		c.setBlock(b)
	}
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
	if round > 0 {
		sortHeight = height - 1
	}
	if sortHeight < 0 {
		sortHeight = 0
	}
	b, ok := c.block_mp[sortHeight]
	if !ok {
		return fmt.Errorf("verifySort error: NO block in the cache. height=%d", height)
	}
	diff := c.difficulty(n, round, height)
	clog.Infow("verifySort", "height", height, "round", round, "diff", diff, "seed", utils.ByteString(b.Hash()))
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

func (c *Consensus) sortition(seed []byte, height int64, round int) {
	amount, err := ycc.QueryDeposit(c.myAddr, c.db)
	if err != nil {
		clog.Errorw("sortition error", "err", err)
		return
	}
	c.sortMaker(height, round, int(amount), seed)
	c.sortCommittee(height, round, int(amount), seed)
}

func (c *Consensus) sortMaker(height int64, round, count int, seed []byte) {
	input := &types.VrfInput{
		Height: height,
		Round:  int32(round),
		Seed:   seed,
	}

	clog.Infow("sortMaker", "height", height, "round", round, "n", count, "seed", utils.ByteString(seed))
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
	clog.Infow("voteMaker", "height", height, "round", round, "nvote", len(myHashs), "hash", utils.ByteString(v.Hash))

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
	clog.Infow("voteCommittee", "height", height, "round", round, "nvote", len(myHashs))

	c.node.Publish(types.CommitteeVoteTopic, cmmtt)
	c.handleCommitteeVote(cmmtt)
}

func (c *Consensus) handleBlocks(m *types.BlocksReply) bool {
	if len(m.Bs) == 0 {
		return false
	}

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

	synced := m.LastHeight == c.lastBlock.Header.Height
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
			panic("GenesisBlock panic" + err.Error())
		}
		err = c.execBlock(gb)
		if err != nil {
			panic("GenesisBlock panic" + err.Error())
		}
		c.addNewBlock(gb)
		return
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
	for i := int64(1); i < int64(c.AdvSortBlocks); i++ {
		c.sortMaker(i, round, int(n), seed)
		c.sortCommittee(i, round, int(n), seed)
		if i < int64(c.AdvVoteBlocks) {
			c.voteMaker(i, round)
			c.voteCommittee(i, round)
		}
	}
}

func (c *Consensus) getPreBlocks(height int64) error {
	clog.Infow("getPreBlocks", "height", height)
	if c.UseRpcToDataNode {
		c.pool.Put(&getPreBlockTask{height: height, c: c})
	} else {
		c.p2pGetBlocks(height, 1, types.GetPreBlockTopic)
	}
	return nil
}

func (c *Consensus) getBlocks(start, count int64) error {
	if c.UseRpcToDataNode {
		c.pool.Put(&getBlocksTask{start: start, count: int(count), c: c})
	} else {
		c.p2pGetBlocks(start, count, types.GetBlocksTopic)
	}
	return nil
}

func (c *Consensus) p2pGetBlocks(start, count int64, topic string) error {
	arg := &types.GetBlocks{
		Start: start,
		Count: count,
	}
	return c.node.Send(c.DataNodePID, topic, arg)
}

func (c *Consensus) setNewBlock(nb *types.NewBlock) error {
	if c.UseRpcToDataNode {
		return c.rpcClt.Call(context.Background(), "SetNewBlock", nb, nil)
	} else {
		return c.node.Send(c.DataNodePID, types.SetNewBlockTopic, nb)
	}
}

// TODO: peerlist and data node list
