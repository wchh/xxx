package log

import (
	"os"

	// "github.com/33cn/chain33/common/log/log15"

	"github.com/sirupsen/logrus"
)

// type Log = log15.Logger

// func New(s ...string) Log {
// 	return log15.New(s)
// }

type Logger = logrus.Entry

func NewLogger(m string) *Logger {
	return logrus.WithField("mode", m)
}

func Set(fpath, level string) {
	file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		logrus.Out = file
	} else {
		logrus.Info("Failed to log to file, using default stderr")
	}
	switch level {
	case "info":
		logrus.Level = logrus.Info
	case "debug":
	case "error":
	case "warning":
	case "panic":
	case "fatal":

	}
}
