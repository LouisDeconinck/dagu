// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	persisfile "github.com/dagucloud/dagu/v2/internal/persis/file"
)

func TestNewDAGRepositoryMigratesLegacySuspendFlags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, "data")
	flagsDir := filepath.Join(dataDir, "suspend")
	legacyDir := filepath.Join(homeDir, "suspend")

	require.NoError(t, os.MkdirAll(legacyDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "alpha.suspend"), []byte{}, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "beta.suspend"), []byte{}, 0o600))

	repo, err := persisfile.NewDAGRepository(
		suspendFlagsTestConfig(homeDir, dataDir, flagsDir, legacyDir),
		persisfile.WithDAGSkipExamples(true),
	)
	require.NoError(t, err)

	for _, id := range []string{"alpha", "beta"} {
		suspended, err := repo.IsSuspended(ctx, id)
		require.NoError(t, err)
		assert.True(t, suspended, "expected %q to stay suspended after migration", id)
	}
	assert.NoFileExists(t, filepath.Join(legacyDir, "alpha.suspend"))
	assert.NoDirExists(t, legacyDir)
}

func TestNewDAGRepositoryMigrationKeepsNewerFlag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, "data")
	flagsDir := filepath.Join(dataDir, "suspend")
	legacyDir := filepath.Join(homeDir, "suspend")

	require.NoError(t, os.MkdirAll(legacyDir, 0o750))
	require.NoError(t, os.MkdirAll(flagsDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "alpha.suspend"), []byte{}, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(flagsDir, "alpha.suspend"), []byte{}, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "beta.suspend"), []byte{}, 0o600))

	repo, err := persisfile.NewDAGRepository(
		suspendFlagsTestConfig(homeDir, dataDir, flagsDir, legacyDir),
		persisfile.WithDAGSkipExamples(true),
	)
	require.NoError(t, err)

	for _, id := range []string{"alpha", "beta"} {
		suspended, err := repo.IsSuspended(ctx, id)
		require.NoError(t, err)
		assert.True(t, suspended)
	}
	assert.NoDirExists(t, legacyDir)
}

func TestNewDAGRepositoryWithoutLegacySuspendFlags(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, "data")
	flagsDir := filepath.Join(dataDir, "suspend")

	repo, err := persisfile.NewDAGRepository(
		suspendFlagsTestConfig(homeDir, dataDir, flagsDir, filepath.Join(homeDir, "suspend")),
		persisfile.WithDAGSkipExamples(true),
	)
	require.NoError(t, err)

	suspended, err := repo.IsSuspended(context.Background(), "alpha")
	require.NoError(t, err)
	assert.False(t, suspended)
}

func suspendFlagsTestConfig(homeDir, dataDir, flagsDir, legacyDir string) *config.Config {
	cfg := &config.Config{}
	cfg.Paths.DAGsDir = filepath.Join(homeDir, "dags")
	cfg.Paths.DataDir = dataDir
	cfg.Paths.SuspendFlagsDir = flagsDir
	cfg.Paths.SuspendFlagsDirLegacy = legacyDir
	return cfg
}
