package consensus

import (
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
)

var clog = log.New("consensus")

type hr struct {
	h int64
	r int
}

type Consensus struct {
	*config.ConsensusConfig
	chid      string
	version   string
	rpcPort   int
	n         *p2p.Node
	db        db.DB
	c         *contract.Container
	lastBlock *types.Block
	priv      crypto.PrivateKey
	myAddr    string

	maker_mp      map[int64]map[int]*maker
	committee_mp  map[int64]map[int]*committee
	preblock_mp   map[int64]*types.Block
	alldeposit_mp map[int64]int
	block_mp      map[int64]*types.Block
	peer_mp       map[string]*types.PeerInfo

	vbch chan hr
	nbch chan *types.Block
	mch  chan *p2p.Tmsg
}

func New(conf *config.Config, c *contract.Container) (*Consensus, error) {
	ldb, err := db.NewLDB(conf.DataPath)
	if err != nil {
		return nil, err
	}
	if conf.Consensus.VotePrice == 0 {
		conf.Consensus.VotePrice = 10000
	}
	priv := crypto.NewKeyFromSeed([]byte(conf.Consensus.PrivateSeed))
	p2pConf := &p2p.Conf{Priv: priv, Port: conf.ServerPort}
	node, err := p2p.NewNode(p2pConf)
	if err != nil {
		return nil, err
	}
	return &Consensus{
		chid:            conf.ChainID,
		version:         conf.Version,
		rpcPort:         conf.RpcPort,
		priv:            priv,
		ConsensusConfig: conf.Consensus,
		n:               node,
		db:              ldb,
		c:               c,
		myAddr:          crypto.PubkeyToAddr(priv.PublicKey()),
		maker_mp:        make(map[int64]map[int]*maker),
		committee_mp:    make(map[int64]map[int]*committee),
		preblock_mp:     make(map[int64]*types.Block),
		alldeposit_mp:   make(map[int64]int),
		block_mp:        make(map[int64]*types.Block),
		peer_mp:         make(map[string]*types.PeerInfo),

		vbch: make(chan hr, 1),
		nbch: make(chan *types.Block, 1),
		mch:  make(chan *p2p.Tmsg, 256),
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

func (c *Consensus) GenesisBlock() (*types.Block, error) {
	tx0, err := coin.CreateIssueTx(coin.CoinX*1e8*100, c.c)
	if err != nil {
		return nil, err
	}
	amount := 10000 * coin.CoinX * c.VotePrice
	tx1, err := coin.CreateTransferTx(nil, c.myAddr, amount, 0)
	if err != nil {
		return nil, err
	}
	tx2, err := ycc.CreateDepositTx(nil, c.myAddr, amount, 0)
	if err != nil {
		return nil, err
	}
	txs := []*types.Tx{tx0, tx1, tx2}
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

func (c *Consensus) handleP2pMsg(msg *p2p.Tmsg) {
	switch msg.Topic {
	case p2p.PreBlockTopic:
		c.handlePreBlock(msg.Msg.(*types.PreBlock))
	case p2p.MakerSortTopic:
		c.handleMakerSort(msg.Msg.(*types.Sortition))
	case p2p.CommitteeSortTopic:
		c.handleCommitteeSort(msg.Msg.(*types.Sortition))
	case p2p.MakerVoteTopic:
		c.handleMakerVote(msg.Msg.(*types.CommitteeVote))
	case p2p.CommitteeVoteTopic:
		c.handleCommitteeVote(msg.Msg.(*types.CommitteeVote))
	case p2p.BlockVoteTopic:
		c.handleBlockVote(msg.Msg.(*types.Vote))
	case p2p.ConsensusBlockTopic:
		c.handleConsensusBlock(msg.Msg.(*types.NewBlock))
	case p2p.BlocksReplyTopic:
		c.handleBlocksReply(msg.Msg.(*types.BlocksReply))
	}
}

func (c *Consensus) readP2pMsg() {
	for msg := range c.n.C {
		tm, err := c.decodeP2pMsg(msg)
		if err != nil {
			clog.Errorw("decdoeP2pMsg error", "err", err)
			continue
		}
		c.mch <- tm
	}
}

func (c *Consensus) decodeP2pMsg(msg *p2p.Msg) (*p2p.Tmsg, error) {
	var umsg types.Message
	switch msg.Topic {
	case p2p.PreBlockTopic:
		var m types.PreBlock
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case p2p.MakerSortTopic:
		var m types.Sortition
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case p2p.CommitteeSortTopic:
		var m types.Sortition
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case p2p.MakerVoteTopic:
		var m types.CommitteeVote
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case p2p.CommitteeVoteTopic:
		var m types.CommitteeVote
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case p2p.BlockVoteTopic:
		var m types.Vote
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case p2p.ConsensusBlockTopic:
		var m types.NewBlock
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	case p2p.BlocksReplyTopic:
		var m types.BlocksReply
		err := types.Unmarshal(msg.Data, &m)
		if err != nil {
			panic(err)
		}
		umsg = &m
	}
	return &p2p.Tmsg{PID: msg.PID, Topic: msg.Topic, Msg: umsg}, nil
}

func (c *Consensus) Run() {
	go runRpc(fmt.Sprintf(":%d", c.rpcPort), c)
	c.sync()
	go c.readP2pMsg()
	c.ConsensusRun()
}

func (c *Consensus) handleNewblock(b *types.Block, round int) {
	c.makeBlock(b, round)
	c.sortition(b, round)
	c.voteMaker(b.Header.Height+int64(c.AdvVoteBlocks), round)
	c.voteCommittee(b.Header.Height+int64(c.AdvVoteBlocks), round)
	c.clean(b.Header.Height)
	c.getPreBlocks(b.Header.Height+1, 4)
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
			c.handleP2pMsg(msg)
		case hr := <-c.vbch:
			c.voteBlock(hr.h, hr.r)
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

	lh := string(c.lastBlock.Hash())
	comm := c.getCommittee(pb.Header.Height, int(pb.Header.Round))
	if (types.Votes)(comm.bvs[lh]).Count() < MustVotes {
		return
	}

	maker := c.getmaker(height, round)
	maker.setMaker()
	if maker.my == nil {
		return
	}
	_, ok := maker.mvmp[string(maker.my.Hashs[0].Hash)]
	if !ok {
		return
	}

	m := &types.Ycc_Mine{
		Votes: comm.bvs[lh],
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
		return
	}
	b.Header.Round = int32(round)
	b.Txs = append([]*types.Tx{tx0}, b.Txs...)

	nb, err := c.perExecBlock(b)
	if err != nil {
		panic(err)
	}

	c.n.Publish(p2p.ConsensusBlockTopic, nb)
	c.handleConsensusBlock(nb)
}

func (c *Consensus) handlePreBlock(b *types.PreBlock) {
	txs := b.B.Txs
	if c.CheckSig {
		txs, _, _ = txsVerifySig(txs, 8, false)
		b.B.Txs = txs
	}
	c.preblock_mp[b.B.Header.Height] = b.B
}

func (c *Consensus) handleMakerVote(v *types.CommitteeVote) {
	maker := c.getmaker(v.Height, int(v.Round))
	for _, h := range v.CommHashs {
		// if !comm.findSort(string(h)) {
		// 	return
		// }
		maker.mvmp[string(h)] += len(v.MyHashs)
	}
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

	txs := b.Txs
	if c.CheckSig {
		txs, failedHash, _ = txsVerifySig(txs, 8, false)
	}

	db := db.NewMDB(c.db)
	c.c.SetDB(db)

	rtxs := make([]*types.Tx, len(txs))
	index := 0
	for _, tx := range txs {
		err := c.c.ExecTx(tx)
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
	t, err := c.db.OpenTransaction()
	if err != nil {
		return err
	}
	kv := db.NewBKV(t)
	c.c.SetDB(kv)

	defer doErr(&err, func() {
		t.Discard()
	})

	txs := b.Txs
	if c.CheckSig {
		txs, _, err = txsVerifySig(txs, 8, true)
		if err != nil {
			return err
		}
	}

	for _, tx := range txs {
		err = c.c.ExecTx(tx)
		if err != nil {
			return err
		}
	}

	err = writeLastHeader(b.Header, kv)
	if err != nil {
		return err
	}

	err = t.Write(kv.B)
	if err != nil {
		return err
	}
	return t.Commit()
}

func (c *Consensus) setBlock(hash string, height int64, round int) {
	comm := c.getCommittee(height, round)
	nb := comm.ab.get(hash)
	if nb == nil {
		return
	}

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
	c.n.Publish(p2p.NewBlockTopic, nb)
}

func (c *Consensus) addNewBlock(b *types.Block) {
	c.lastBlock = b // b is complete block
	c.block_mp[b.Header.Height] = b
	go func() {
		c.nbch <- b
	}()
}

func (c *Consensus) handleBlockVote(v *types.Vote) {
	comm := c.getCommittee(v.Height, int(v.Round))
	comm.bvs[string(v.Hash)] = append(comm.bvs[string(v.Hash)], v)

	if !comm.ok && len(comm.bvs[string(v.Hash)]) >= MustVotes {
		c.setBlock(string(v.Hash), v.Height, int(v.Round))
		comm.ok = true
	}
}

func (c *Consensus) handleConsensusBlock(b *types.NewBlock) {
	if c.lastBlock.Header.Height >= b.Header.Height {
		return
	}
	height := b.Header.Height
	round := int(b.Header.Round)
	comm := c.getCommittee(height, round)
	comm.ab.add(b)
	if comm.ab.len() == 1 {
		time.AfterFunc(time.Millisecond*500, func() {
			c.vbch <- hr{height, round}
		})
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
		all = int(n / c.VotePrice)
		c.alldeposit_mp[h] = all
	}

	allf := float64(all) * math.Pow(0.9, float64(r))
	diff := float64(t) / allf
	return diff
}

func (c *Consensus) verifySort(s *types.Sortition) error {
	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	b, ok := c.block_mp[height-int64(c.AdvSortBlocks)]
	if !ok {
		return fmt.Errorf("verifySort error: NO block in the cache. height=%d", height)
	}
	in := &types.VrfInput{Height: height, Round: int32(round), Seed: b.Hash()}
	if err := vrfVerify(in, s, c.difficulty(MakerSize, round, height)); err != nil {
		return err
	}
	return nil
}

func (c *Consensus) handleMakerSort(s *types.Sortition) {
	err := c.verifySort(s)
	if err != nil {
		clog.Errorw("handleMakerSort error:", "err", err)
	}

	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	comm := c.getCommittee(height, round)
	comm.mss[string(s.Sig.PublicKey)] = s
}

func (c *Consensus) handleCommitteeSort(s *types.Sortition) {
	err := c.verifySort(s)
	if err != nil {
		clog.Errorw("handleCommitteeSort error:", "err", err)
	}

	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	comm := c.getCommittee(height, round)
	comm.css[string(s.Sig.PublicKey)] = s
}

func (c *Consensus) sortition(b *types.Block, round int) {
	n, err := ycc.QueryDeposit(c.myAddr, c.db)
	if err != nil {
		clog.Errorw("sortition error", "err", err)
		return
	}
	height := b.Header.Height + int64(c.AdvSortBlocks)
	seed := b.Hash()
	c.sortMaker(height, round, int(n/c.VotePrice), seed)
	c.sortCommittee(height, round, int(n/c.VotePrice), seed)
}

func (c *Consensus) sortMaker(height int64, round, n int, seed []byte) {
	input := &types.VrfInput{
		Height: height,
		Round:  int32(round),
		Seed:   seed,
	}

	s, err := vrfSortiton(input, n, 1, c.difficulty(MakerSize, round, height))
	if err != nil {
		clog.Errorw("sortCommittee error", "err", err)
		return
	}
	if len(s.Hashs) == 0 {
		return
	}

	// only 1 hash
	sort.Sort(types.SortHashs(s.Hashs))
	s.Hashs = s.Hashs[:1]
	s.Sign(c.priv)
	c.handleMakerSort(s)

	c.n.Publish(p2p.MakerSortTopic, s)
}

func (c *Consensus) sortCommittee(height int64, round, n int, seed []byte) {
	input := &types.VrfInput{
		Height: height,
		Round:  int32(round),
		Seed:   seed,
	}

	s, err := vrfSortiton(input, n, 3, c.difficulty(CommitteeSize, round, height))
	if err != nil {
		clog.Errorw("sortCommittee error", "err", err)
		return
	}
	if len(s.Hashs) == 0 {
		return
	}

	s.Sign(c.priv)
	c.handleCommitteeSort(s)

	c.n.Publish(p2p.CommitteeSortTopic, s)
}

func (c *Consensus) voteMaker(height int64, round int) {
	comm := c.getCommittee(height, round)
	myHashs, _ := comm.getMyHashs()
	if myHashs == nil {
		return
	}
	v := &types.CommitteeVote{
		Height:    height,
		Round:     int32(round),
		CommHashs: comm.getMakerHashs(),
		MyHashs:   myHashs,
	}

	c.n.Publish(p2p.MakerVoteTopic, v)
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

	c.n.Publish(p2p.CommitteeVoteTopic, cmmtt)
	c.handleCommitteeVote(cmmtt)
}

func (c *Consensus) voteBlock(height int64, round int) {
	comm := c.getCommittee(height, round)
	sort.Sort(types.NewBlockSlice(comm.ab.bs))

	myhashs, _ := comm.getMyHashs()
	if myhashs == nil {
		return
	}
	if len(comm.ab.bs) > 0 {
		b := comm.ab.bs[0]
		v := &types.Vote{
			Height:  height,
			Round:   int32(round),
			Hash:    b.Header.TxsHash,
			MyHashs: myhashs,
		}
		v.Sign(c.priv)
		c.n.Publish(p2p.BlockVoteTopic, v)
		c.handleBlockVote(v)
	}
}

func (c *Consensus) handleBlocksReply(m *types.BlocksReply) bool {
	for _, b := range m.Bs {
		if b.Header.Height != c.lastBlock.Header.Height+1 {
			return false
		}
		err := c.execBlock(b)
		if err != nil {
			clog.Errorw("execBlock error", "err", err)
			return false
		}
		c.lastBlock = b
	}

	return m.LastHeight == m.Bs[len(m.Bs)-1].Header.Height
}

func (c *Consensus) getPreBlocks(start, count int64) error {
	for i := start; i < start+count; i++ {
		_, ok := c.preblock_mp[i]
		if !ok {
			c.getBlocks(i, 1, p2p.GetPreBlocksTopic)
		}
	}
	return nil
}

func (c *Consensus) getBlocks(start, count int64, m string) error {
	r := &types.GetBlocks{
		Start: start,
		Count: count,
	}
	p := c.getDataNode()
	if p != nil {
		return c.n.Send(p.Pid, m, r)
	}
	return fmt.Errorf("NO data node")
}

func (c *Consensus) getDataNode() *types.PeerInfo {
	for _, p := range c.peer_mp {
		if p.NodeType == "data" {
			return p
		}
	}
	return nil
}

func (c *Consensus) sync() {
	lastHeader, err := readLastHeader(c.db)
	if err != nil {
		clog.Errorw("readLastHeader error", "err", err)
		gb, err := c.GenesisBlock()
		if err != nil {
			clog.Panicw("GenesisBlock panic", "err", err)
		}
		err = c.execBlock(gb)
		if err != nil {
			clog.Panicw("GenesisBlock panic", "err", err)
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
		br, err := c.rpcGetBlocks(c.lastBlock.Header.Height+1, 10, "GetBlocks")
		if err != nil {
			clog.Errorw("sync error", "err", err)
		}
		synced = c.handleBlocksReply(br)
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
	for i := int64(0); i < int64(c.AdvSortBlocks)+1; i++ {
		c.sortMaker(i, round, int(n/c.VotePrice), seed)
		c.sortCommittee(i, round, int(n/c.VotePrice), seed)
		if i < int64(c.AdvVoteBlocks) {
			c.voteMaker(i, round)
			c.voteCommittee(i, round)
		}
	}
}

// TODO: peerlist and data node list
