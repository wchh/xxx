package vrf

import (
	"testing"
)

func TestE(t *testing.T) {
	in := []byte("12345asdfhasdfjalsdfasdfhlasdfjalsdf")
	sk, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	hash, proof := sk.Evaluate(in)
	if !sk.Public().Verify(hash[:], in, proof) {
		t.Error("vrf verify error")
	}
}
