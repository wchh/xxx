package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func Init(path, level string) {
	wr := zapcore.AddSync(os.Stdout)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		wr = zapcore.NewMultiWriteSyncer(wr, zapcore.AddSync(file))
	} else {
		zap.S().DPanic(err)
	}
	atom := zap.NewAtomicLevelAt(conv_level(level))
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(newEncoderConfig()),
		wr,
		atom,
	)
	logger := zap.New(core, zap.AddCaller(), zap.Development())
	zap.ReplaceGlobals(logger)
}

func Sync() {
	zap.L().Sync()
}

type Logger = zap.SugaredLogger

func New(m string) *Logger {
	return zap.S().With("mod", m)
}

func conv_level(level string) zapcore.Level {
	switch level {
	case "info":
		return zap.InfoLevel
	case "debug":
		return zap.DebugLevel
	case "error":
		return zap.ErrorLevel
	case "warning":
		return zap.WarnLevel
	case "panic":
		return zap.PanicLevel
	case "fatal":
		return zap.FatalLevel
	}
	return zap.InfoLevel
}
