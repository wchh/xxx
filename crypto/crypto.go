package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/33cn/chain33/common/address"
	"github.com/tendermint/tendermint/crypto/merkle"
)

type PrivateKey ed25519.PrivateKey
type PublicKey ed25519.PublicKey

func NewKey() (PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	return PrivateKey(priv), err
}

func NewKeyFromSeed(seed []byte) PrivateKey {
	priv := ed25519.NewKeyFromSeed(seed)
	return PrivateKey(priv)
}

func Sign(priv PrivateKey, msg []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(priv), msg)
}

func Verify(pub PublicKey, msg, sig []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(pub), msg, sig)
}

func (priv PrivateKey) PublicKey() PublicKey {
	publicKey := make([]byte, ed25519.PublicKeySize)
	copy(publicKey, priv[32:])
	return publicKey
}

func (priv PrivateKey) String() string {
	return hex.EncodeToString(priv)
}

func PrivateKeyFromString(s string) (PrivateKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, errors.New("privatekey len error")
	}
	return PrivateKey(b), nil
}

func Hash(msg []byte) []byte {
	h := sha256.Sum256(msg)
	return h[:]
}

func DoubleHash(msg []byte) []byte {
	return Hash(Hash(msg))
}

func (pub PublicKey) Address() string {
	return address.PubKeyToAddr(pub)
}

func NewAddress(hash []byte) string {
	return address.HashToAddress(0, hash).String()
}

func PubkeyToAddr(pub PublicKey) string {
	return address.PubKeyToAddr(pub)
}

func Merkle(hashs [][]byte) []byte {
	return merkle.HashFromByteSlices(hashs)
}
