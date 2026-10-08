package jev_svc

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/opskat/opskat/internal/bootstrap"
	"github.com/opskat/opskat/internal/service/credential_svc"
	"github.com/stretchr/testify/require"
)

var testDataDir string

func TestMain(m *testing.M) {
	var err error
	testDataDir, err = os.MkdirTemp("", "opskat-jev-settings-test-")
	if err != nil {
		panic(err)
	}
	if _, err := bootstrap.LoadConfig(testDataDir); err != nil {
		panic(err)
	}
	credential_svc.SetDefault(credential_svc.New("isolated-test-key", []byte("0123456789abcdef")))
	code := m.Run()
	if err := os.RemoveAll(testDataDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}

func TestAPIKeyEncryptedPersistenceUpdateAndRemoval(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, SaveAPIKey(ctx, "jev-test-original-key"))
	data, err := os.ReadFile(filepath.Join(testDataDir, "config.json")) //nolint:gosec // Path is created by this package's isolated TestMain.
	require.NoError(t, err)
	require.NotContains(t, string(data), "jev-test-original-key")
	key, err := APIKey(ctx)
	require.NoError(t, err)
	require.Equal(t, "jev-test-original-key", key)
	require.Error(t, SaveAPIKey(ctx, "invalid\nkey"))
	key, err = APIKey(ctx)
	require.NoError(t, err)
	require.Equal(t, "jev-test-original-key", key)
	require.NoError(t, SaveAPIKey(ctx, "replacement-key"))
	key, err = APIKey(ctx)
	require.NoError(t, err)
	require.Equal(t, "replacement-key", key)
	require.NoError(t, SaveAPIKey(ctx, ""))
	key, err = APIKey(ctx)
	require.NoError(t, err)
	require.Empty(t, key)
	bootstrap.GetConfig().JevAPIKey = "corrupt-ciphertext"
	_, err = APIKey(ctx)
	require.Error(t, err, "decryption failure must surface")
}

func TestPrimaryConfidenceThresholdDefaultsAndPersists(t *testing.T) {
	ctx := context.Background()
	bootstrap.GetConfig().JevPrimaryConfidenceThreshold = 0
	require.Equal(t, 0.5, PrimaryConfidenceThreshold())
	require.NoError(t, SavePrimaryConfidenceThreshold(ctx, 0.73))
	require.InDelta(t, 0.73, PrimaryConfidenceThreshold(), 0.000001)
	require.InDelta(t, 0.73, bootstrap.GetConfig().JevPrimaryConfidenceThreshold, 0.000001)

	for _, value := range []float64{math.NaN(), math.Inf(1), 0.49, 1.01} {
		require.Error(t, SavePrimaryConfidenceThreshold(ctx, value))
	}
}
