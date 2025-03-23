package logger

import (
	"go.uber.org/zap"
)

var Log *zap.Logger = zap.NewNop()

func Init(level string) error {
	l, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = l
	logger, err := cfg.Build()
	Log = logger
	if err != nil {
		return err
	}
	return nil
}
