package consensus

import (
	"errors"
	"fmt"
	"math/big"

	"xxx/crypto"
	"xxx/crypto/vrf"
	"xxx/types"
)

type sortTask struct {
	vrfHash []byte
	index   int
	group   int
	diff    float64
	ch      chan<- *types.SortHash
}

func sortFunc(vrfHash []byte, index, group int, diff float64) *types.SortHash {
	data := fmt.Sprintf("%x+%d+%d", vrfHash, index, group)
	hash := crypto.DoubleHash([]byte(data))

	// 转为big.Float计算，比较难度diff
	y := new(big.Int).SetBytes(hash)
	z := new(big.Float).SetInt(y)
	if new(big.Float).Quo(z, fmax).Cmp(big.NewFloat(diff)) > 0 {
		return nil
	}
	return &types.SortHash{Index: int64(index), Hash: hash, Group: int32(group)}
}

func (t *sortTask) Do() {
	t.ch <- sortFunc(t.vrfHash, t.index, t.group, t.diff)
}

var max = big.NewInt(0).Exp(big.NewInt(2), big.NewInt(256), nil)
var fmax = big.NewFloat(0).SetInt(max) // 2^^256
func (c *Consensus) vrfSortiton(input *types.VrfInput, count, group int, diff float64) (*types.Sortition, error) {
	in, err := types.Marshal(input)
	if err != nil {
		return nil, err
	}

	vrfSk := (*vrf.PrivateKey)(&c.priv)
	vrfHash, proof := vrfSk.Evaluate(in)
	vrfProof := &types.VrfProof{Input: input, Proof: proof, Hash: vrfHash[:], PublicKey: vrfSk.Public()}

	ch := make(chan *types.SortHash, 8)
	go func() {
		for j := 0; j < group; j++ {
			for i := 0; i < count; i++ {
				c.pool.Put(&sortTask{vrfHash: vrfHash[:], index: i, group: j, diff: diff, ch: ch})
			}
		}
	}()
	var ss []*types.SortHash
	k := 0
	for k < count*group {
		sh := <-ch
		if sh != nil {
			ss = append(ss, sh)
		}
		k++
	}
	close(ch)
	return &types.Sortition{Proof: vrfProof, Hashs: ss}, nil
}

func vrfVerify(input *types.VrfInput, s *types.Sortition, diff float64) error {
	in, err := types.Marshal(s.Proof.Input)
	if err != nil {
		return err
	}
	in2, err := types.Marshal(input)
	if err != nil {
		return err
	}
	if string(in) != string(in2) {
		return errors.New("vrf input NOT match")
	}

	pk := (vrf.PublicKey)(s.Proof.PublicKey)
	index, err := pk.PoofToHash(in, s.Proof.Proof)
	if err != nil {
		return errors.New("vrf verify failed" + err.Error())
	}
	if string(index[:]) != string(s.Proof.Hash) {
		return errors.New("vrf verify failed: index NOT same")
	}

	for _, h := range s.Hashs {
		data := fmt.Sprintf("%x+%d+%d", s.Proof.Hash, h.Index, h.Group)
		hash := crypto.DoubleHash([]byte(data))

		// 转为big.Float计算，比较难度diff
		y := new(big.Int).SetBytes(hash)
		z := new(big.Float).SetInt(y)
		if new(big.Float).Quo(z, fmax).Cmp(big.NewFloat(diff)) > 0 {
			return errors.New("vrf calcaulate hash error")
		}
	}
	return nil
}
