// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// pipeStdin replaces process stdin with a pipe holding input until the test
// ends. Not parallel-safe: it swaps the process-global os.Stdin.
func pipeStdin(t *testing.T, input string) {
	t.Helper()

	stdin, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin = original
		require.NoError(t, stdin.Close())
	})
	_, err = writer.WriteString(input)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
}

func TestStdinHasParamsInput(t *testing.T) {
	t.Run("Pipe", func(t *testing.T) {
		pipeStdin(t, "P1=foo")
		require.True(t, stdinHasParamsInput())
	})

	t.Run("RedirectedFile", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "params.txt")
		require.NoError(t, os.WriteFile(path, []byte("P1=foo"), 0o600))
		file, err := os.Open(path)
		require.NoError(t, err)
		original := os.Stdin
		os.Stdin = file
		t.Cleanup(func() {
			os.Stdin = original
			require.NoError(t, file.Close())
		})
		require.True(t, stdinHasParamsInput())
	})

	t.Run("CharacterDevice", func(t *testing.T) {
		// A character device (terminal or /dev/null) is never params input, so
		// interactive runs and runs detached from stdin are not read.
		devNull, err := os.Open(os.DevNull)
		require.NoError(t, err)
		original := os.Stdin
		os.Stdin = devNull
		t.Cleanup(func() {
			os.Stdin = original
			require.NoError(t, devNull.Close())
		})
		require.False(t, stdinHasParamsInput())
	})
}

func TestReadStdinParams(t *testing.T) {
	t.Run("TrimsWhitespace", func(t *testing.T) {
		pipeStdin(t, "  P1=foo P2=bar\n")
		params, err := readStdinParams()
		require.NoError(t, err)
		require.Equal(t, "P1=foo P2=bar", params)
	})

	t.Run("Empty", func(t *testing.T) {
		pipeStdin(t, "")
		params, err := readStdinParams()
		require.NoError(t, err)
		require.Empty(t, params)
	})
}
