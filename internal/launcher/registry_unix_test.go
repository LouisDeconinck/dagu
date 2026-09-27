// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package launcher_test

import (
	"context"
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
