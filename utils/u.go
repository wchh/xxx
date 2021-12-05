package utils

import (
	"bytes"
	"encoding/hex"
	"io"

	"github.com/pierrec/lz4"
)

func Compress(data []byte) []byte {
	r := bytes.NewReader(data)
	b := new(bytes.Buffer)
	w := lz4.NewWriter(b)
	io.Copy(w, r)
	w.Close()
	return b.Bytes()
}

func Uncompress(data []byte) []byte {
	r := lz4.NewReader(bytes.NewReader(data))
	b := new(bytes.Buffer)
	io.Copy(b, r)
	return b.Bytes()
}

type Bytes []byte

func (b Bytes) String() string {
	nb := b
	if len(nb) > 8 {
		nb = nb[:8]
	}
	return hex.EncodeToString(nb)
}
