package vrf

import (
	"errors"
	"xxx/crypto"
)

type PrivateKey crypto.PrivateKey
type PublicKey crypto.PublicKey

func (sk PrivateKey) Evaluate(m []byte) (index [32]byte, proof []byte) {
	h := crypto.DoubleHash(m)
	copy(index[:], h)
	proof = crypto.Sign(crypto.PrivateKey(sk), h)
	return
}

func (priv PrivateKey) Public() PublicKey {
	return (PublicKey)(priv[32:])
}

func (pk PublicKey) PoofToHash(m, proof []byte) (index [32]byte, err error) {
	h := crypto.DoubleHash(m)
	if !crypto.Verify(crypto.PublicKey(pk), h, proof) {
		err = errors.New("vevify failed")
		return
	}
	copy(index[:], h)
	return
}

// func NewKey() (PrivateKey, error) {
// 	_, sk, err := vrf.GenerateKey(rand.Reader)
// 	return sk[:], err
// }

// func NewKeyFromSeed(seed []byte) (PrivateKey, error) {
// 	r := bytes.NewBuffer(seed)
// 	_, sk, err := vrf.GenerateKey(r)
// 	return sk[:], err
// }
