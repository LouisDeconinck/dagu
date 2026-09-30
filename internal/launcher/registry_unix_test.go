// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package launcher_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dagucloud/dagu/v2/internal/launcher"
)

// The registry signals the whole process group, so children spawned by the
// launched command terminate too. A shell running a nested sleep proves the
// signal reached past the directly started process.
func TestProcessRegistryPropagatesToProcessGroup(t *testing.T) {
	reg := launcher.NewProcessRegistry()
	ctx := launcher.ContextWithProcessRegistry(context.Background(), reg)

	res, err := launcher.StartProcess(ctx, launcher.CmdSpec{
		Executable: "sh",
		Args:       []string{"-c", "sleep 30"},
	})
	require.NoError(t, err)

	reg.Propagate(ctx, syscall.SIGTERM)

	select {
	case <-res.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("propagated signal did not terminate the process group")
	}

	// Once every group member exited, probing the empty group fails with ESRCH.
	require.Eventually(t, func() bool {
		return syscall.Kill(-res.PID, 0) == syscall.ESRCH
	}, 3*time.Second, 50*time.Millisecond)
}

// Cancellation must leave a registered run alive so shutdown can forward the
// actual signal and the run can complete its own cleanup.
func TestRunCancellationWithRegistry(t *testing.T) {
	reg := launcher.NewProcessRegistry()
	ctx, cancel := context.WithCancel(launcher.ContextWithProcessRegistry(t.Context(), reg))
	defer cancel()
	ready := filepath.Join(t.TempDir(), "ready")
	stopped := filepath.Join(t.TempDir(), "stopped")
	done := make(chan error, 1)
	go func() {
		done <- launcher.Run(ctx, launcher.CmdSpec{
			Executable: "sh",
			Args:       []string{"-c", `trap 'printf TERM > "$2"; exit 0' TERM; printf ready > "$1"; while :; do sleep 0.05; done`, "probe", ready, stopped},
		})
	}()
	t.Cleanup(func() {
		reg.Propagate(context.Background(), syscall.SIGKILL)
	})
	require.Eventually(t, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case err := <-done:
		t.Fatalf("cancellation terminated the registered run: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	reg.Propagate(context.Background(), syscall.SIGTERM)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("registered run did not finish after SIGTERM")
	}
	data, err := os.ReadFile(stopped)
	require.NoError(t, err)
	require.Equal(t, "TERM", string(data))
}

func TestRunCancellationWithoutRegistry(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready := filepath.Join(t.TempDir(), "ready")
	done := make(chan error, 1)
	go func() {
		done <- launcher.Run(ctx, launcher.CmdSpec{
			Executable: "sh",
			Args:       []string{"-c", `printf ready > "$1"; exec sleep 30`, "probe", ready},
		})
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not stop the unregistered run")
	}
}
