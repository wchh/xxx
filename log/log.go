package log

import (
	"os"
	"path/filepath"

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
	lv := conv_level(level)
	encoderConf := newEncoderConfig()
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConf)
	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), lv)

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		os.MkdirAll(path, os.ModePerm)
	}
	fp := filepath.Join(path, "log.txt")
	file, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	encoderConf.EncodeLevel = zapcore.CapitalLevelEncoder
	fileEncoder := zapcore.NewConsoleEncoder(encoderConf)
	fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(file), lv)

	logger := zap.New(zapcore.NewTee(consoleCore, fileCore), zap.AddCaller()) //, zap.Development())
	zap.ReplaceGlobals(logger)

	for m, l := range logger_map {
		*l = *zap.S().With("mod", m)
	}
}

func Sync() {
	zap.L().Sync()
}

type Logger = zap.SugaredLogger

var logger_map map[string]*Logger

func Register(m string, logger *Logger) {
	if logger_map == nil {
		logger_map = make(map[string]*zap.SugaredLogger)
	}
	logger_map[m] = logger
}

func New(m string) *Logger {
	logger := new(Logger)
	Register(m, logger)
	return logger
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
