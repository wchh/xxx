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

func BenchmarkSign(b *testing.B) {
	msg := []byte("adfaldfjadlsfladsfhalsdfjalsdfjalsdfasdhflsjfasdfjlas")
	sk, _ := NewKey()
	for i := 0; i < b.N; i++ {
		Sign(sk, msg)
	}
}

func BenchmarkVerigy(b *testing.B) {
	msg := []byte("adfaldfjadlsfladsfhalsdfjalsdfjalsdfasdhflsjfasdfjlas")
	sk, _ := NewKey()
	sig := Sign(sk, msg)
	pk := sk.PublicKey()

	for i := 0; i < b.N; i++ {
		Verify(pk, msg, sig)
	}
}
