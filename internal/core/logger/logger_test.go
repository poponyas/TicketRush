package core_logger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestChildCloseDoesNotCloseRootFile(t *testing.T) {
	folder := t.TempDir()
	log, err := NewLogger(Config{LogLevel: "debug", Folder: folder})
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })

	child := log.With(zap.String("request_id", "request-1"))
	require.NoError(t, child.Close())
	child.Info("child message")
	log.Info("root message")
	require.NoError(t, log.Close())
	require.NoError(t, log.Close())

	files, err := filepath.Glob(filepath.Join(folder, "*.log"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.Contains(t, string(data), "child message")
	require.Contains(t, string(data), "request-1")
	require.Contains(t, string(data), "root message")
}

func TestFromContextWithoutLogger(t *testing.T) {
	for _, ctx := range []context.Context{
		context.Background(),
		ToContext(context.Background(), nil),
	} {
		require.NotPanics(t, func() {
			log := FromContext(ctx)
			log.Info("fallback message")
			require.NoError(t, log.Close())
		})
	}
}

func TestNewConfigMustPanicsOnInvalidConfig(t *testing.T) {
	t.Setenv("LOGGER_FOLDER", t.TempDir())
	t.Setenv("FOLDER", "")
	t.Setenv("LOGGER_LEVEL", "debug")
	// The required folder must be absent, rather than merely empty.
	require.NoError(t, os.Unsetenv("LOGGER_FOLDER"))
	require.NoError(t, os.Unsetenv("FOLDER"))
	require.Panics(t, func() { NewConfigMust() })
}
