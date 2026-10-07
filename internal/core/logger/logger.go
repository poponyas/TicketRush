package core_logger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.Logger

	file      *os.File
	closeOnce sync.Once
	closeErr  error
}

type loggerContextKey struct{}

var key = loggerContextKey{}

var noopLogger = &Logger{Logger: zap.NewNop()}

func ToContext(ctx context.Context, log *Logger) context.Context {
	return context.WithValue(
		ctx,
		key,
		log,
	)
}

func FromContext(ctx context.Context) *Logger {
	log, ok := ctx.Value(key).(*Logger)
	if !ok || log == nil || log.Logger == nil {
		return noopLogger
	}

	return log
}

func NewLogger(cfg Config) (*Logger, error) {
	if strings.TrimSpace(cfg.Folder) == "" {
		return nil, fmt.Errorf("logger folder must not be empty")
	}
	logLevel, err := zapcore.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("can't parse level: %w", err)
	}

	if err := os.MkdirAll(cfg.Folder, 0755); err != nil {
		return nil, fmt.Errorf("can't mkdir for logger.Folder: %w", err)
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05.000000")

	folderPath := filepath.Join(
		cfg.Folder,
		fmt.Sprintf("%s.log", timestamp),
	)

	logFile, err := os.OpenFile(folderPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	zapCfg := zap.NewDevelopmentEncoderConfig()
	zapCfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02T15-04-05.000000")

	zapEncoder := zapcore.NewConsoleEncoder(zapCfg)

	core := zapcore.NewTee(
		zapcore.NewCore(zapEncoder, zapcore.AddSync(os.Stdout), logLevel),
		zapcore.NewCore(zapEncoder, zapcore.AddSync(logFile), logLevel),
	)

	zapLogger := zap.New(core, zap.AddCaller())
	return &Logger{
		Logger: zapLogger,
		file:   logFile,
	}, nil
}

// Close flushes and closes the file owned by the root logger.
// Loggers returned by With do not own the file; their Close is a no-op.
// Stop producers of logs before closing the root logger.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.closeOnce.Do(func() {
		// Sync only the file: stdout may not support fsync.
		l.closeErr = errors.Join(l.file.Sync(), l.file.Close())
	})
	return l.closeErr
}

func (l *Logger) With(field ...zap.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(field...),
	}
}
