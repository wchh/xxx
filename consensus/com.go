package consensus

import (
	"sort"

	"xxx/db"
	"xxx/types"
)

type alterBlock struct {
	// n  int
	bs []*types.NewBlock
}

func (ab *alterBlock) get(h string) *types.NewBlock {
	for _, b := range ab.bs {
		if h == string(b.Header.TxsHash) {
			return b
		}
	}
	return nil
}

func (ab *alterBlock) len() int {
	return len(ab.bs)
}

func (ab *alterBlock) add(nb *types.NewBlock) bool {
	for _, b := range ab.bs {
		if string(b.Header.TxsHash) == string(nb.Header.TxsHash) {
			return false
		}
	}
	ab.bs = append(ab.bs, nb)
	return true
}

// 区块制作人
type maker struct {
	my   *types.Sortition // 我的抽签
	mvmp map[string]int   // 我收到的 maker vote
	// c        *Consensus
	// selected bool
	// ok       bool
}

// 验证委员会
type committee struct {
	myss *types.Sortition            // 我的抽签
	mss  map[string]*types.Sortition // 我接收到的maker 的抽签
	css  map[string]*types.Sortition // 我收到committee的抽签
	ssmp map[string]*types.SortHash
	svmp map[string]int // 验证委员会的投票
	ab   *alterBlock    // 我接收到所有备选block
	bvs  map[string][]*types.Vote
	ok   bool // 表示区块已共识
	// voteOk bool // 表示已经投票
	b  *types.Block
	db db.Transaction
}

func (c *Consensus) getmaker(height int64, round int) *maker {
	rmp, ok := c.maker_mp[height]
	if !ok {
		rmp = make(map[int]*maker)
		c.maker_mp[height] = rmp
	}
	v, ok := rmp[round]
	if !ok {
		v = &maker{
			mvmp: make(map[string]int),
		}
		rmp[round] = v
	}
	return v
}

const (
	MakerSize     = 7
	MustVotes     = 17
	CommitteeSize = 25
)

func (m *maker) setMaker() {
	for k, n := range m.mvmp {
		if n < MustVotes {
			delete(m.mvmp, k)
		}
	}
	// clog.Info("committee len", "len", len(c.svmp), "height", height)
}

func (c *committee) setCommittee() {
	for k, n := range c.svmp {
		if n < MustVotes {
			delete(c.svmp, k)
		}
	}
	// clog.Info("committee len", "len", len(c.svmp), "height", height)
}

// func (c *committee) myCommitteeSort() []*types.SortHash {
// 	var mss []*types.SortHash
// 	for _, h := range c.myss.Hashs {
// 		_, ok := c.svmp[string(h.Hash)]
// 		if ok {
// 			mss = append(mss, h)
// 		}
// 	}
// 	if len(mss) == 0 {
// 		return c.getMySorts()
// 	}
// 	return mss
// }

func (c *committee) findSort(h string) bool {
	for _, s := range c.css {
		for _, ha := range s.Hashs {
			if h == string(ha.Hash) {
				return true
			}
		}
	}
	return false
}

func (c *Consensus) getCommittee(height int64, round int) *committee {
	rmp, ok := c.committee_mp[height]
	if !ok {
		rmp = make(map[int]*committee)
		c.committee_mp[height] = rmp
	}
	m, ok := rmp[round]
	if !ok {
		m = &committee{
			mss:  make(map[string]*types.Sortition),
			css:  make(map[string]*types.Sortition),
			ssmp: make(map[string]*types.SortHash),
			svmp: make(map[string]int),
			bvs:  make(map[string][]*types.Vote),
			ab:   &alterBlock{},
		}
		rmp[round] = m
	}
	return m
}

/*
func getSorts(mp map[string]*types.Sortition, num int) []*types.SortHash {
	var ss []*types.SortHash
	for _, s := range mp {
		ss = append(ss, s.Hashs...)
	}
	if len(ss) > num {
		sort.Sort(types.SortHashs(ss))
		ss = ss[:num]
	}
	return ss
}
*/
// func (m *maker) checkVotes(height int64, vs []*types.Vote) (int, error) {
// 	if height > 0 && len(vs) < MustVotes {
// 		return 0, errors.New("checkVotes error: NOT enough votes")
// 	}
// 	return len(vs), nil
// }

// func (c *committee) checkVotes(vs []*types.Vote) (int, error) {
// 	if len(vs) < MustVotes {
// 		return 0, errors.New("checkVotes error: NOT enough votes")
// 	}
// 	return len(vs), nil
// }

func (c *committee) getMyHashs(hs [][]byte) [][]byte {
	var mysh [][]byte
	for _, h := range hs {
		for _, mh := range c.myss.Hashs {
			if string(h) == string(mh.Hash) {
				mysh = append(mysh, h)
			}
		}
	}
	return mysh
}

func (c *committee) getMakerHashs() [][]byte {
	var ss []*types.SortHash
	for _, s := range c.mss {
		ss = append(ss, s.Hashs...)
	}

	num := 3
	if len(ss) > num {
		sort.Sort(types.SortHashs(ss))
		ss = ss[:num]
	}

	var hs [][]byte
	for _, s := range ss {
		hs = append(hs, s.Hash)
	}
	return hs
}

func (c *committee) getCommitteeHashs() [][]byte {
	var ss []*types.SortHash
	for _, s := range c.css {
		ss = append(ss, s.Hashs...)
	}

	num := CommitteeSize
	if len(ss) > num {
		sort.Sort(types.SortHashs(ss))
		ss = ss[:num]
	}

	var hs [][]byte
	for _, s := range ss {
		hs = append(hs, s.Hash)
	}
	return hs
}

// func (c *committee) getCommitteeSorts() map[string]*types.SortHash {
// 	if len(c.ssmp) > 0 {
// 		return c.ssmp
// 	}
// 	// c.checkSors()
// 	num := CommitteeSize

// 	var ss []*types.SortHash
// 	for _, s := range c.css {
// 		ss = append(ss, s.Hashs...)
// 	}
// 	if len(ss) > num {
// 		sort.Sort(types.SortHashs(ss))
// 		ss = ss[:num]
// 	}

// 	for _, s := range ss {
// 		c.ssmp[string(s.Hash)] = s
// 	}
// 	return c.ssmp
// }

// func (c *committee) getMySorts() []*types.SortHash {
// 	ssmp := c.getCommitteeSorts()
// 	var ss []*types.SortHash
// 	for _, s := range ssmp {
// 		for _, mh := range c.myss.Hashs {
// 			if string(mh.Hash) == string(s.Hash) {
// 				ss = append(ss, mh)
// 			}
// 		}
// 	}
// 	return ss
// }

// func (m *maker) findVm(key, pub string) bool {
// 	return find(m.mvs, key, pub)
// }

// func (m *committee) findVb(key, pub string) bool {
// 	return find(m.bvs, key, pub)
// }

// func find(vmp map[string][]*types.Vote, key, pub string) bool {
// 	vs, ok := vmp[key]
// 	if !ok {
// 		return false
// 	}
// 	for _, v := range vs {
// 		if string(v.Sig.PublicKey) == pub {
// 			return true
// 		}
// 	}
// 	return false
// }
