package utils

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"xxx/crypto"
)

func TestCompress(t *testing.T) {
	data := []byte("abcdefg")
	cb := Compress(data)
	ub := Uncompress(cb)

	if !bytes.Equal(data, ub) {
		t.Error("error")
	}
}

type task struct {
	pk     crypto.PublicKey
	m, sig []byte
	ch     chan bool
}

func (t *task) Do() {
	t.ch <- crypto.Verify(t.pk, t.m, t.sig)
}

func TestGoPool(t *testing.T) {
	pool := NewPool(8, 100)
	const count = 30000
	ts := make([]*task, count)
	m := []byte("asdfalsdfalsdfladfalsdfalsdfladfjaldfajlsdf")
	sk, _ := crypto.NewKey()
	pk := sk.PublicKey()
	ch := make(chan bool, count)
	for i := 0; i < count; i++ {
		ts[i] = &task{pk: pk, m: m, sig: crypto.Sign(sk, m), ch: ch}
	}
	go pool.Run()

	bt := time.Now()
	go func() {
		for _, t := range ts {
			pool.Put(t)
		}
	}()
	k := 0
	for range ch {
		k++
		if k == count {
			break
		}
	}
	close(ch)
	fmt.Println("verify", count, "用时", time.Since(bt))
}
