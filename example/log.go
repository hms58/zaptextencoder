package main

import (
	"os"
	"zaptextencoder"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Level         zapcore.Level
	ColorfulLevel bool
}

var logger *zap.Logger
var sugaredLogger *zap.SugaredLogger
var _sugaredLogger *zap.SugaredLogger

// New 构造Logger对象
func New(cfg *Config) error {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "message",
		LevelKey:       "level",
		TimeKey:        "time",
		NameKey:        "logger",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if cfg.ColorfulLevel {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	encoder := zaptextencoder.NewTextEncoder(encoderCfg)
	//encoder := zapcore.NewConsoleEncoder(encoderCfg)
	//encoder := zapcore.NewJSONEncoder(encoderCfg)

	cores := []zapcore.Core{
		zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), cfg.Level),
	}
	core := zapcore.NewTee(cores...)
	pid := zap.Fields(zap.Int("pid", os.Getpid()))
	caller := zap.AddCaller()
	callerSkip := zap.AddCallerSkip(1)
	stacktrace := zap.AddStacktrace(zapcore.ErrorLevel)

	logger = zap.New(core, pid, caller, stacktrace)
	sugaredLogger = logger.Sugar()
	_sugaredLogger = zap.New(core, pid, caller, callerSkip, stacktrace).Sugar()
	//zap.ReplaceGlobals(logger)
	return nil
}

// Debug logger
func Debug(args ...interface{}) {
	_sugaredLogger.Debug(args...)
}

// Info logger
func Info(args ...interface{}) {
	_sugaredLogger.Info(args...)
}

// Warn logger
func Warn(args ...interface{}) {
	_sugaredLogger.Warn(args...)
}

// Error logger
func Error(args ...interface{}) {
	_sugaredLogger.Error(args...)
}

// Fatal logger
func Fatal(args ...interface{}) {
	_sugaredLogger.Fatal(args...)
}

// Debugf logger
func Debugf(template string, args ...interface{}) {
	_sugaredLogger.Debugf(template, args...)
}

// Infof logger
func Infof(template string, args ...interface{}) {
	_sugaredLogger.Infof(template, args...)
}

// Warnf logger
func Warnf(template string, args ...interface{}) {
	_sugaredLogger.Warnf(template, args...)
}

// Errorf logger
func Errorf(template string, args ...interface{}) {
	_sugaredLogger.Errorf(template, args...)
}

// Fatalf logger
func Fatalf(template string, args ...interface{}) {
	_sugaredLogger.Fatalf(template, args...)
}

// Panicf logger
func Panicf(template string, args ...interface{}) {
	_sugaredLogger.Panicf(template, args...)
}

// Debugw logger
func Debugw(msg string, keysAndValues ...interface{}) {
	_sugaredLogger.Debugw(msg, keysAndValues...)
}

// Infow logger
func Infow(msg string, keysAndValues ...interface{}) {
	_sugaredLogger.Infow(msg, keysAndValues...)
}

// Warnw logger
func Warnw(msg string, keysAndValues ...interface{}) {
	_sugaredLogger.Warnw(msg, keysAndValues...)
}

// Errorw logger
func Errorw(msg string, keysAndValues ...interface{}) {
	_sugaredLogger.Errorw(msg, keysAndValues...)
}

// Fatalw logger
func Fatalw(msg string, keysAndValues ...interface{}) {
	_sugaredLogger.Fatalw(msg, keysAndValues...)
}

// Panicw logger
func Panicw(msg string, keysAndValues ...interface{}) {
	_sugaredLogger.Panicw(msg, keysAndValues...)
}
