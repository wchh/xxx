package utils

import (
	"bytes"
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
