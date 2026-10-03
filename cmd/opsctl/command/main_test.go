package command

import (
	"os"
	"testing"

	"github.com/opskat/opskat/internal/bootstrap"
)

func TestMain(m *testing.M) {
	dataDir, err := os.MkdirTemp("", "opskat-command-test-")
	if err != nil {
		panic(err)
	}
	if _, err := bootstrap.LoadConfig(dataDir); err != nil {
		panic(err)
	}
	code := m.Run()
	if err := os.RemoveAll(dataDir); err != nil {
		panic(err)
	}
	os.Exit(code)
}
