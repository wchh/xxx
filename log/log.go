package log

import (
	"github.com/33cn/chain33/common/log/log15"
)

type Log = log15.Logger

func New(s ...string) Log {
	return log15.New(s)
}
