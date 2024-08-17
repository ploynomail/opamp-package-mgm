package opamppackagemgm

import (
	"errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	ErrHashMismatch      = errors.New("new file hash mismatch after patch")
	defaultHTTPRequester = &HTTPRequester{}
)

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

type Loggerr interface {
	Log(zapcore.Level, string, ...zapcore.Field)
}

type defaultLog struct {
	logger *zap.Logger
}

func (l *defaultLog) Log(level zapcore.Level, format string, value ...zapcore.Field) {
	switch level {
	case zapcore.DebugLevel:
		l.logger.Debug(format, value...)
	case zapcore.InfoLevel:
		l.logger.Info(format, value...)
	case zapcore.WarnLevel:
		l.logger.Warn(format, value...)
	case zapcore.ErrorLevel:
		l.logger.Error(format, value...)
	}
}

func NewLog() Loggerr {
	return &defaultLog{
		logger: zap.NewExample(),
	}
}

type Info struct {
	Version string
	Sha256  []byte
	IsPatch bool
}
