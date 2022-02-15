package consensus

import (
	"sort"

	"xxx/db"
	"xxx/types"
)

// 区块制作人
type maker struct {
	my   *types.Sortition         // 我的抽签
	mvmp map[string][]*types.Vote // 我收到的 maker vote
}

// 验证委员会
type committee struct {
	myss *types.Sortition            // 我的抽签
	mss  map[string]*types.Sortition // 我接收到的maker 的抽签
	css  map[string]*types.Sortition // 我收到committee的抽签
	ssmp map[string]*types.SortHash
	svmp map[string]int // 验证委员会的投票
	ok   bool           // 表示区块已共识
	b    *types.Block
	db   db.Transaction
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
			mvmp: make(map[string][]*types.Vote),
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
	for k, v := range m.mvmp {
		if len(v) < MustVotes {
			delete(m.mvmp, k)
		}
	}
}

func (c *committee) setCommittee() {
	for k, n := range c.svmp {
		if n < MustVotes {
			delete(c.svmp, k)
		}
	}
}

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
		}
		rmp[round] = m
	}
	return m
}

func (c *committee) getMyHashs() ([][]byte, [][]byte) {
	comHashs := c.getCommitteeHashs()
	if c.myss == nil {
		return nil, comHashs
	}
	var mysh [][]byte
	for _, h := range comHashs {
		for _, mh := range c.myss.Hashs {
			if string(h) == string(mh.Hash) {
				mysh = append(mysh, h)
			}
		}
	}
	return mysh, comHashs
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
