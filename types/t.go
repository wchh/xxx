package types

import (
	"encoding/hex"
	"encoding/json"
	"xxx/crypto"

	"github.com/gogo/protobuf/proto"
	// "google.golang.org/protobuf/proto"
)

type Message = proto.Message

func Marshal(msg proto.Message) ([]byte, error) {
	return proto.Marshal(msg)
}

func Unmarshal(buf []byte, msg Message) error {
	return proto.Unmarshal(buf, msg)
}

func (tx *Tx) Hash() []byte {
	msg, _ := Marshal(tx)
	return msg
}

func (tx *Tx) Sign(priv crypto.PrivateKey) {
	tx.Sig = nil
	sig := crypto.Sign(priv, tx.Hash())
	tx.Sig = &Signature{PublicKey: priv.PublicKey(), Signature: sig}
}

func (tx *Tx) Verify() bool {
	sig := tx.Sig
	tx.Sig = nil
	r := crypto.Verify(sig.PublicKey, tx.Hash(), sig.Signature)
	tx.Sig = sig
	return r
}

func (h *Header) Hash() []byte {
	data, _ := Marshal(h)
	return crypto.Hash(data)
}

func (b *Block) Hash() []byte {
	return b.Header.Hash()
}

func (s *Sortition) Sign(priv crypto.PrivateKey) {
	s.Sig = nil
	buf, err := Marshal(s)
	if err != nil {
		panic(err)
	}
	sig := crypto.Sign(priv, buf)
	s.Sig = &Signature{PublicKey: priv.PublicKey(), Signature: sig}
}

func (s *Sortition) Verify() bool {
	sig := s.Sig
	s.Sig = nil
	buf, err := Marshal(s)
	if err != nil {
		panic(err)
	}
	ok := crypto.Verify(sig.PublicKey, buf, sig.Signature)
	s.Sig = sig
	return ok
}

func (v *Vote) Sign(priv crypto.PrivateKey) {
	v.Sig = nil
	buf, err := Marshal(v)
	if err != nil {
		panic(err)
	}
	sig := crypto.Sign(priv, buf)
	v.Sig = &Signature{PublicKey: priv.PublicKey(), Signature: sig}
}

func (v *Vote) Verify() bool {
	sig := v.Sig
	v.Sig = nil
	buf, err := Marshal(v)
	if err != nil {
		panic(err)
	}
	ok := crypto.Verify(sig.PublicKey, buf, sig.Signature)
	v.Sig = sig
	return ok
}

type SortHashs []*SortHash

func (m SortHashs) Len() int { return len(m) }
func (m SortHashs) Less(i, j int) bool {
	if m[i].Group < m[j].Group {
		return true
	}
	return string(m[i].Hash) < string(m[j].Hash)
}
func (m SortHashs) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

type Votes []*Vote

func (m Votes) Count() int {
	sum := 0
	for _, v := range ([]*Vote)(m) {
		sum += len(v.MyHashs)
	}
	return sum
}

type NewBlockSlice []*NewBlock

func (m NewBlockSlice) Len() int { return len(m) }
func (m NewBlockSlice) Less(i, j int) bool {
	return string(m[i].Header.TxsHash) < string(m[j].Header.TxsHash)
}
func (m NewBlockSlice) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

func (v *CommitteeVote) Sign(sk crypto.PrivateKey) {
	v.Sig = nil
	buf, err := Marshal(v)
	if err != nil {
		panic(err)
	}
	sig := crypto.Sign(sk, buf)
	v.Sig = &Signature{PublicKey: sk.PublicKey(), Signature: sig}
}

func (v *CommitteeVote) Verify() bool {
	sig := v.Sig
	v.Sig = nil
	buf, err := Marshal(v)
	if err != nil {
		panic(err)
	}
	ok := crypto.Verify(sig.PublicKey, buf, sig.Signature)
	v.Sig = sig
	return ok
}

func (nb *NewBlock) Sign(sk crypto.PrivateKey) {
	nb.Sig = nil
	buf, err := Marshal(nb)
	if err != nil {
		panic(err)
	}
	sig := crypto.Sign(sk, buf)
	nb.Sig = &Signature{PublicKey: sk.PublicKey(), Signature: sig}
}

func (nb *NewBlock) Verify() bool {
	sig := nb.Sig
	nb.Sig = nil
	buf, err := Marshal(nb)
	if err != nil {
		panic(err)
	}
	ok := crypto.Verify(sig.PublicKey, buf, sig.Signature)
	nb.Sig = sig
	return ok
}

func JsonString(m Message) string {
	b, _ := json.MarshalIndent(m, "", "\t")
	return string(b)
}

type Hash []byte

func (h Hash) String() string {
	return hex.EncodeToString(h)
}

func HashFromString(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

func UnmarshalAmount(data []byte) (int64, error) {
	a := new(Amount)
	err := Unmarshal(data, a)
	if err != nil {
		return 0, err
	}
	return a.A, nil
}

type ServerInfo = PeerInfo

func TxsMerkel(txs []*Tx) []byte {
	var hashs [][]byte
	for _, tx := range txs {
		hashs = append(hashs, tx.Hash())
	}
	return crypto.Merkle(hashs)
}
