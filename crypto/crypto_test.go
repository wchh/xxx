package crypto

import (
	"bytes"
	"fmt"
	"testing"
)

func TestNewKeyFromSeed(t *testing.T) {
	sk, err := NewKey()
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Println("sk:", sk)
	fmt.Println("pk:", sk.PublicKey())

	fsk := NewKeyFromSeed([]byte(sk[:32]))
	fmt.Println("pk:", fsk.PublicKey())
	if !bytes.Equal(fsk.PublicKey(), sk.PublicKey()) {
		t.Error("NewKeyFromSeed error")
	}
}
