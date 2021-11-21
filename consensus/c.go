package consensus

import (
	"fmt"
	"math"
	"sort"
	"time"

	"xxx/contract"
	"xxx/contract/ycc"
	"xxx/crypto"
	"xxx/db"
	"xxx/log"
	"xxx/p2p"
	"xxx/types"
)

var clog = log.New("consensus")

type Conf struct {
	PrivateSeed   string
	DataPath      string
	ServerAddr    string
	RpcAddr       string
	VotePrice     int64
	GenesisAddr   string
	TxFee         int64
	AdvSortBlocks int
	AdvVoteBlocks int
}

type hr struct {
	h int64
	r int
}

type Consensus struct {
	*Conf
	n         *p2p.Node
	db        db.DB
	c         *contract.Container
	lastBlock *types.Block
	priv      crypto.PrivateKey

	maker_mp      map[int64]map[int]*maker
	committee_mp  map[int64]map[int]*committee
	preblock_mp   map[int64]*types.Block
	alldeposit_mp map[int64]int
	blocks_mp     map[int64]*types.Block

	vbch chan hr
	nbch chan *types.Block
}

func New(conf *Conf, n *p2p.Node, c *contract.Container) (*Consensus, error) {
	ldb, err := db.NewLDB(conf.DataPath)
	if err != nil {
		return nil, err
	}
	if conf.VotePrice == 0 {
		conf.VotePrice = 10000
	}
	return &Consensus{
		Conf:          conf,
		n:             n,
		db:            ldb,
		c:             c,
		maker_mp:      make(map[int64]map[int]*maker),
		committee_mp:  make(map[int64]map[int]*committee),
		preblock_mp:   make(map[int64]*types.Block),
		alldeposit_mp: make(map[int64]int),
		blocks_mp:     make(map[int64]*types.Block),
		vbch:          make(chan hr, 1),
		nbch:          make(chan *types.Block, 1),
	}, nil
}

func (c *Consensus) GenesisBlock() *types.Block {
	return nil
}

func (c *Consensus) handleP2pMsg(m *p2p.Msg) {
	switch m.Topic {
	case p2p.PreBlockTopic:
		var v types.Block
		err := types.Unmarshal(m.Data, &v)
		if err != nil {
			panic(err)
		}
		c.handlePreBlock(&v)
	case p2p.MakerSortTopic:
		var v types.Sortition
		err := types.Unmarshal(m.Data, &v)
		if err != nil {
			panic(err)
		}
		c.handleMakerSort(&v)
	case p2p.CommitteeSortTopic:
		var v types.Sortition
		err := types.Unmarshal(m.Data, &v)
		if err != nil {
			panic(err)
		}
		c.handleCommitteeSort(&v)
	case p2p.MakerVoteTopic:
		var v types.CommitteeVote
		err := types.Unmarshal(m.Data, &v)
		if err != nil {
			panic(err)
		}
		c.handleMakerVote(&v)
	case p2p.CommitteeVoteTopic:
		var v types.CommitteeVote
		err := types.Unmarshal(m.Data, &v)
		if err != nil {
			panic(err)
		}
		c.handleCommitteeVote(&v)
	case p2p.BlockVoteTopic:
		var v types.Vote
		err := types.Unmarshal(m.Data, &v)
		if err != nil {
			panic(err)
		}
		c.handleBlockVote(&v)
	case p2p.ConsensusBlockTopic:
		var b types.NewBlock
		err := types.Unmarshal(m.Data, &b)
		if err != nil {
			panic(err)
		}
		c.handleConsensusBlock(&b)
	case p2p.BlocksTopic:
		var b types.BlocksReply
		err := types.Unmarshal(m.Data, &b)
		if err != nil {
			panic(err)
		}
		c.handleBlocksReply(m.PID, &b)
	}
}

func (c *Consensus) Run() {
	go runRpc(c.RpcAddr, c)
	c.ConsensusRun()
}

func (c *Consensus) ConsensusRun() {
	round := 0
	blockTimeout := time.Second * 4
	tm1Ch := make(chan int64, 1)
	tm2Ch := make(chan int64, 1)
	tm3Ch := make(chan int64, 1)

	for {
		select {
		case msg := <-c.n.C:
			c.handleP2pMsg(msg)
		case hr := <-c.vbch:
			c.voteBlock(hr.h, hr.r)
		case b := <-c.nbch:
			round = 0
			c.makeBlock(b, round)
			c.sortition(b, round)
			c.voteMaker(b.Header.Height+int64(c.AdvVoteBlocks), round)
			c.voteCommittee(b.Header.Height+int64(c.AdvVoteBlocks), round)
			time.AfterFunc(blockTimeout, func() { tm1Ch <- b.Header.Height })
		case <-tm3Ch:
			c.makeBlock(c.lastBlock, round)
		case height := <-tm2Ch:
			c.voteMaker(height, round)
			c.voteCommittee(height, round)
			time.AfterFunc(time.Second, func() { tm2Ch <- height })
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
	b := c.preblock_mp[height]
	b.Header.Round = int32(round)
	txs := b.Txs
	b.Txs = []*types.Tx{tx0}
	b.Txs = append(b.Txs, txs...)

	nb, err := c.perExecBlock(b)
	if err != nil {
		panic(err)
	}

	c.n.Publish(p2p.ConsensusBlockTopic, nb)
	c.handleConsensusBlock(nb)
}

func (c *Consensus) handlePreBlock(b *types.Block) {
	c.preblock_mp[b.Header.Height] = b
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

func (c *Consensus) perExecBlock(b *types.Block) (*types.NewBlock, error) {
	var failedHash [][]byte

	db := db.NewMDB(c.db)
	c.c.SetDB(db)

	txs := make([]*types.Tx, len(b.Txs))
	index := 0
	for _, tx := range b.Txs {
		err := c.c.ExecTx(tx)
		if err != nil {
			failedHash = append(failedHash, tx.Hash())
		} else {
			txs[index] = tx
		}
		index++
	}
	txs = txs[:index]
	b.Header.TxsHash = txsMerkel(txs)
	b.Txs = txs

	comm := c.getCommittee(b.Header.Height, int(b.Header.Round))
	comm.db = db
	comm.b = b
	return &types.NewBlock{Header: b.Header, FailedHashs: failedHash, Tx0: b.Txs[0]}, nil
}

func txsMerkel(txs []*types.Tx) []byte {
	var hashs [][]byte
	for _, tx := range txs {
		hashs = append(hashs, tx.Hash())
	}
	return crypto.Merkle(hashs)
}

func (c *Consensus) execBlock(b *types.Block) error {
	t, err := c.db.OpenTransaction()
	if err != nil {
		return err
	}
	kv := db.NewBKV(t)
	c.c.SetDB(kv)

	for _, tx := range b.Txs {
		err := c.c.ExecTx(tx)
		if err != nil {
			t.Discard()
			return err
		}
	}
	err = t.Write(kv.B)
	if err != nil {
		t.Discard()
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

		if string(nb.Header.TxsHash) != string(txsMerkel(txs)) {
			panic("merkel NOT same")
		}

		b = &types.Block{Header: nb.Header, Txs: txs}
		err := c.execBlock(b)
		if err != nil {
			panic(err)
		}
		comm.b = b
	}

	c.lastBlock = b // b is complete block
	c.blocks_mp[b.Header.Height] = b
	c.n.Publish(p2p.NewBlockTopic, nb)
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
			clog.Error("diff error", "err", err)
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
	b, ok := c.blocks_mp[height-10]
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
		clog.Error("handleMakerSort error:", "err", err)
	}

	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	comm := c.getCommittee(height, round)
	comm.mss[string(s.Sig.PublicKey)] = s
}

func (c *Consensus) handleCommitteeSort(s *types.Sortition) {
	err := c.verifySort(s)
	if err != nil {
		clog.Error("handleCommitteeSort error:", "err", err)
	}

	height := s.Proof.Input.Height
	round := int(s.Proof.Input.Round)
	comm := c.getCommittee(height, round)
	comm.css[string(s.Sig.PublicKey)] = s
}

func (c *Consensus) sortition(b *types.Block, round int) {
	n, err := ycc.QueryDeposit(crypto.PubkeyToAddr(c.priv.PublicKey()), c.db)
	if err != nil {
		clog.Error("sortition error", "err", err)
		return
	}
	height := b.Header.Height + int64(c.AdvSortBlocks)
	seed := b.Hash()
	c.sortMaker(height, round, int(n/c.VotePrice), seed)
	c.sortCommittee(height, round, int(n), seed)
}

func (c *Consensus) sortMaker(height int64, round, n int, seed []byte) {
	input := &types.VrfInput{
		Height: height,
		Round:  int32(round),
		Seed:   seed,
	}

	s, err := vrfSortiton(input, n, 1, c.difficulty(MakerSize, round, height))
	if err != nil {
		clog.Error("sortCommittee error", "err", err)
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
		clog.Error("sortCommittee error", "err", err)
		return
	}
	s.Sign(c.priv)
	c.handleCommitteeSort(s)

	c.n.Publish(p2p.CommitteeSortTopic, s)
}

func (c *Consensus) voteMaker(height int64, round int) {
	comm := c.getCommittee(height, round)
	hashs := comm.getCommitteeHashs()
	v := &types.CommitteeVote{
		Height:    height,
		Round:     int32(round),
		CommHashs: comm.getMakerHashs(),
		MyHashs:   comm.getMyHashs(hashs),
	}

	c.n.Publish(p2p.MakerVoteTopic, v)
	c.handleMakerVote(v)
}

func (c *Consensus) voteCommittee(height int64, round int) {
	comm := c.getCommittee(height, round)
	hashs := comm.getCommitteeHashs()
	cmmtt := &types.CommitteeVote{
		Height:    height,
		Round:     int32(round),
		CommHashs: hashs,
		MyHashs:   comm.getMyHashs(hashs),
	}

	c.n.Publish(p2p.CommitteeVoteTopic, cmmtt)
	c.handleCommitteeVote(cmmtt)
}

func (c *Consensus) voteBlock(height int64, round int) {
	comm := c.getCommittee(height, round)
	sort.Sort(types.NewBlockSlice(comm.ab.bs))
	if len(comm.ab.bs) > 0 {
		b := comm.ab.bs[0]
		v := &types.Vote{
			Height:  height,
			Round:   int32(round),
			Hash:    b.Header.TxsHash,
			MyHashs: comm.getMyHashs(comm.getCommitteeHashs()),
		}
		v.Sign(c.priv)
		c.n.Publish(p2p.BlockVoteTopic, v)
		c.handleBlockVote(v)
	}
}

func (c *Consensus) handleBlocksReply(pid string, m *types.BlocksReply) {
	for _, b := range m.Bs {
		for _, tx := range b.Txs {
			err := c.c.ExecTx(tx)
			if err != nil {
				panic(err)
			}
		}
		c.lastBlock = b
	}
	if m.LastHeight != m.Bs[len(m.Bs)-1].Header.Height {
		count := m.LastHeight - c.lastBlock.Header.Height
		if count > 10 {
			count = 10
		}
		r := &types.GetBlocks{
			Start: c.lastBlock.Header.Height + 1,
			Count: count,
		}
		c.n.SendMsg(pid, p2p.GetBlocksTopic, r)
	}
}

func (c *Consensus) Sync() {
}

// TODO: peerlist and data node list
