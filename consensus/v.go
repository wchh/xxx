package consensus

import (
	"errors"
	"sync"
	"xxx/types"
)

func txsVerifySig(txs []*types.Tx, cpuNum int, errReturn bool) ([]*types.Tx, [][]byte, error) {
	ch := make(chan *types.Tx)
	done := make(chan struct{})
	errch := make(chan error, 1)
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
						if errReturn {
							select {
							case errch <- errors.New("tx verify error"):
							default:
							}
						}
					} else {
						tch <- tx
					}
				}
			}
		}()
	}

	wg2 := new(sync.WaitGroup)
	wg2.Add(2)
	var rtxs []*types.Tx
	go func() {
		for tx := range tch {
			rtxs = append(rtxs, tx)
		}
		wg2.Done()
	}()

	var rhs [][]byte
	go func() {
		for h := range hch {
			rhs = append(rhs, h)
		}
		wg2.Done()
	}()

	done2 := make(chan struct{})
	go func() {
		defer close(done)
		for _, tx := range txs {
			select {
			case <-done2:
				return
			default:
			}
			ch <- tx
		}
	}()

	var err error
	select {
	case err = <-errch:
		if errReturn {
			close(done2)
		}
	case <-done:
	}

	wg.Wait()
	close(tch)
	close(hch)
	wg2.Wait()
	return rtxs, rhs, err
}
