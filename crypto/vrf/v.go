package vrf

import (
	"bytes"
	"crypto/rand"

	"github.com/yahoo/coname/vrf"
)

type PrivateKey []byte
type PublicKey []byte

func (priv PrivateKey) Evaluate(m []byte) (index [32]byte, proof []byte) {
	var sk [vrf.SecretKeySize]byte
	copy(sk[:], priv)
	hash, p := vrf.Prove(m, &sk)
	copy(index[:], hash)
	proof = p
	return
}

func (priv PrivateKey) Public() PublicKey {
	return (PublicKey)(priv[32:])
}

func (p PublicKey) Verify(h, m, proof []byte) bool {
	return vrf.Verify(p, m, h, proof)
}

func NewKey() (PrivateKey, error) {
	_, sk, err := vrf.GenerateKey(rand.Reader)
	return sk[:], err
}

func NewKeyFromSeed(seed []byte) (PrivateKey, error) {
	r := bytes.NewBuffer(seed)
	_, sk, err := vrf.GenerateKey(r)
	return sk[:], err
}
