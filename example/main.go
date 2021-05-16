package main

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg := &Config{
		Level:         zapcore.DebugLevel,
		ColorfulLevel: true,
	}
	if err := New(cfg); err != nil {
		log.Fatal(err)
	}
	a := []int{1, 1}
	s := struct {
		Key string
	}{
		Key: "string",
	}
	Info("test", "string")
	Infof("test %s", "string")
	Infof("test %v", map[string]int{"key": 1})
	Infof("test %v", s)
	Infow("test", "key", "string")
	Infow("test", "key", map[string]int{"key": 1})
	Infow("test", "key", s)
	logger.Info("test", zap.String("key", "string\n"))
	logger.Info("test", zap.Int("key", 1))
	logger.Info("test", zap.Any("key", a))
	logger.With(zap.String("module", "testmod")).Info("test", zap.String("key", "string"))
	sugaredLogger.Info("test", "string")
	Debug("test", "string")
	Errorf("error: %v", os.ErrNotExist)
}
