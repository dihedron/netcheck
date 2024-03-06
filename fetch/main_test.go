package fetch

import (
	"log/slog"
	"os"
	"testing"
)

// TestMain sets up the logger for the testing session in this package.
func TestMain(m *testing.M) {
	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level:     slog.LevelDebug,
				AddSource: true,
			}),
		),
	)
	os.Exit(m.Run())
}
