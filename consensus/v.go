package consensus

import (
	"errors"
	"sync"
	"xxx/types"
)

func txsVerifySig(txs []*types.Tx, cpuNum int, errReturn bool) ([]*types.Tx, [][]byte, error) {
	ch := make(chan *types.Tx)
	done := make(chan struct{})
	errch := make(chan struct{}, 1)
	tch := make(chan *types.Tx, 1)
	hch := make(chan []byte, 1)
	wg := new(sync.WaitGroup)
	for i := 0; i < cpuNum; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case tx := <-ch:
					if !tx.Verify() {
						hch <- tx.Hash()
						select {
						case errch <- struct{}{}:
							return
						default:
						}
					} else {
						tch <- tx
					}
				}
			}
		}()
	}

	var rtxs []*types.Tx
	go func() {
		for tx := range tch {
			rtxs = append(rtxs, tx)
		}
	}()

	var rhs [][]byte
	go func() {
		for h := range hch {
			rhs = append(rhs, h)
		}
	}()

	ch2 := make(chan *types.Tx)
	done2 := make(chan struct{})
	go func() {
		for _, tx := range txs {
			ch2 <- tx
		}
		close(done2)
	}()
	for {
		stop := false
		select {
		case <-errch:
			if errReturn {
				return nil, nil, errors.New("tx verify error")
			}
		case <-done2:
			stop = true
		case ch <- <-ch2:
		}
		if stop {
			break
		}
	}
	close(ch)
	wg.Wait()
	close(tch)
	close(hch)
	return rtxs, rhs, nil
}
