package consensus

import (
	"errors"
	"fmt"
	"math/big"

	"xxx/crypto"
	"xxx/crypto/vrf"
	"xxx/types"
)

var max = big.NewInt(0).Exp(big.NewInt(2), big.NewInt(256), nil)
var fmax = big.NewFloat(0).SetInt(max) // 2^^256
func vrfSortiton(input *types.VrfInput, n, g int, diff float64) (*types.Sortition, error) {
	in, err := types.Marshal(input)
	if err != nil {
		return nil, err
	}

	vrfSk, err := vrf.NewKey()
	if err != nil {
		return nil, err
	}

	vrfHash, proof := vrfSk.Evaluate(in)
	vrfProof := &types.VrfProof{Input: input, Proof: proof, Hash: vrfHash[:], PublicKey: vrfSk.Public()}

	var ss []*types.SortHash
	for j := 0; j < g; j++ {
		for i := 0; i < n; i++ {
			data := fmt.Sprintf("%x+%d+%d", vrfHash, i, j)
			hash := crypto.DoubleHash([]byte(data))

			// 转为big.Float计算，比较难度diff
			y := new(big.Int).SetBytes(hash)
			z := new(big.Float).SetInt(y)
			if new(big.Float).Quo(z, fmax).Cmp(big.NewFloat(diff)) > 0 {
				continue
			}
			sh := &types.SortHash{Index: int64(i), Hash: hash, Group: int32(g)}
			ss = append(ss, sh)
		}
	}
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

	publicKey := (vrf.PublicKey)(s.Proof.PublicKey)
	if !publicKey.Verify(s.Proof.Hash, in, s.Proof.Proof) {
		return errors.New("vrf verify failed")
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
