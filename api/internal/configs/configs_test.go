package configs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigEnvironmentOverridesEnvFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("DB_HOST=database-from-file\nACCOUNT_ID=r2-account\n"),
		0o600,
	))
	t.Setenv("DB_HOST", "postgres")

	config, err := LoadConfig(dir)

	require.NoError(t, err)
	require.Equal(t, "postgres", config.DBHost)
	require.Equal(t, "r2-account", config.AccountID)
}
