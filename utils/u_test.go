package utils

import (
	"bytes"
	"testing"
)

func TestCompress(t *testing.T) {
	data := []byte("abcdefg")
	cb := Compress(data)
	ub := Uncompress(cb)

	if !bytes.Equal(data, ub) {
		t.Error("error")
	}
}
