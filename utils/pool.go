package utils

type Task interface {
	Do()
}

// type task struct {
// 	a, b int
// 	c    string
// 	d    chan string
// }

// func (t *task) Do() {
// 	f := func(a, b int, c string, d chan<- string) {
// 		d <- c + fmt.Sprintf("-%d", a+b)
// 	}
// 	f(t.a, t.b, t.c, t.d)
// }

type GoPool struct {
	size int // goroutine number
	tch  chan Task
}

func NewPool(size int) *GoPool {
	if size <= 2 {
		size = 8
	}
	if size > 64 {
		size = 64
	}
	return &GoPool{size, make(chan Task, size)}
}

func (p *GoPool) Run() {
	for i := 0; i < p.size; i++ {
		go func(tch <-chan Task) {
			for t := range tch {
				t.Do()
			}
		}(p.tch)
	}
}
func (p *GoPool) Put(t Task) {
	p.tch <- t
}