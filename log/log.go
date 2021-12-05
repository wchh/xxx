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
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(newEncoderConfig()),
		wr,
		conv_level(level),
	)
	logger := zap.New(core, zap.AddCaller()) //, zap.Development())
	zap.ReplaceGlobals(logger)

	for m, l := range logger_map {
		*l = *newLog(m)
	}
}

func Sync() {
	zap.L().Sync()
}

func newLog(m string) *Logger {
	return zap.S().With("mod", m)
}

type Logger = zap.SugaredLogger

var logger_map map[string]*Logger

func Register(m string, logger *Logger) {
	if logger_map == nil {
		logger_map = make(map[string]*zap.SugaredLogger)
	}
	logger_map[m] = logger
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
