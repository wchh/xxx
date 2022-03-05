package utils

import "sync"

type Task interface {
	Do()
}

type worker struct {
	wch chan<- *worker
	tch chan Task
}

func (w *worker) put(t Task) {
	w.tch <- t
}

func (w *worker) run() {
	for t := range w.tch {
		t.Do()
		if w.wch != nil {
			w.wch <- w
		}
	}
}

type GoPool struct {
	size int // goroutine number
	cap  int
	tch  chan Task
	wch  chan *worker
}

func NewPool(size, cap int) *GoPool {
	if size <= 2 {
		size = 8
	}
	if size > 64 {
		size = 64
	}
	if cap < size {
		cap = size * 2
	}
	return &GoPool{size: size, cap: cap, tch: make(chan Task, size), wch: make(chan *worker, cap)}
}

func (p *GoPool) Run() {
	for i := 0; i < p.size; i++ {
		w := &worker{p.wch, make(chan Task)}
		p.wch <- w
		go w.run()
	}
	for t := range p.tch {
		w, ok := <-p.wch
		if !ok {
			if p.size < p.cap {
				w = &worker{p.wch, make(chan Task)}
				go w.run()
				p.size++
			}
		}
		if w == nil { // size == cap, pool is full, only waiting
			w = <-p.wch
		}
		w.put(t)
	}
}

func (p *GoPool) Put(t Task) {
	p.tch <- t
}

// if you want the same lable tasks run on a same goroutine
type GoLablePool struct {
	sync.Map
}

func (p *GoLablePool) Put(t Task, l interface{}) {
	w, ok := p.Load(l)
	if !ok {
		w = &worker{tch: make(chan Task, 64)}
		p.Store(l, w)
		go w.(*worker).run()
	}
	w.(*worker).put(t)
}
