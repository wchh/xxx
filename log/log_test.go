package log

import (
	"testing"

	"go.uber.org/zap"
)

func TestLogger(t *testing.T) {
	Init("test.log", "debug")
	defer Sync()
	logger := zap.S()
	logger.Debug("debug", "1", 1)
	logger.Debugw("debug", "1", 1)
	logger.Info("info", "1", 1)
	logger.Infow("info", "1", 1)
}
