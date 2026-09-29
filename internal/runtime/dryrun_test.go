// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	cmnvalue "github.com/dagucloud/dagu/v2/internal/cmn/value"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/runctx"
	"github.com/dagucloud/dagu/v2/internal/runtime"
	"github.com/stretchr/testify/require"
)

func TestCheckDryRunStep(t *testing.T) {
	missing := "dagu-test-missing-9f3c2b1a"

	t.Run("MissingShell", func(t *testing.T) {
		step := newStep("1", withCommand("true"), withShell(missing+"-shell"))

		err := checkDryRun(t, t.TempDir(), step, nil)
		require.ErrorContains(t, err, missing+"-shell")
	})

	t.Run("MissingCommandNamesFieldPath", func(t *testing.T) {
		if goruntime.GOOS == "windows" {
			t.Skip("lookup fixtures rely on POSIX paths")
		}
		// The test binary is an executable that exists on every platform.
		self, err := os.Executable()
		require.NoError(t, err)
		step := newStep("1", withShell("direct"))
		step.Commands = []ir.CommandEntry{{Command: self}, parseCommand(missing + "-second")}

		err = checkDryRun(t, t.TempDir(), step, nil)
		require.ErrorContains(t, err, "run[1]")
		require.ErrorContains(t, err, missing+"-second")
	})

	t.Run("NonExecutableCommandPath", func(t *testing.T) {
		if goruntime.GOOS == "windows" {
			t.Skip("executable permission bits are not meaningful on Windows")
		}
		workDir := t.TempDir()
		script := filepath.Join(workDir, "no-exec.sh")
		require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o644))

		err := checkDryRun(t, workDir, newStep("1", withCommand("./no-exec.sh")), nil)
		require.ErrorContains(t, err, "not an executable file")
	})

	t.Run("ShellBuiltin", func(t *testing.T) {
		if windowsShellTest() {
			t.Skip("Windows default shells are not checked for command names")
		}

		err := checkDryRun(t, t.TempDir(), newStep("1", withCommand("exit 0")), nil)
		require.NoError(t, err)
	})

	t.Run("NonLocalExecutor", func(t *testing.T) {
		step := newStep("1", withCommand(missing+"-command"), withShell(missing+"-shell"))
		step.ExecutorConfig.Type = "docker"

		// Docker executors resolve the shell inside the image, not on the host.
		err := checkDryRun(t, t.TempDir(), step, nil)
		require.NoError(t, err)
	})
}

// checkDryRun runs the dry-run executable check for step with workDir as the
// DAG working directory and envs layered over the step environment.
func checkDryRun(t *testing.T, workDir string, step ir.Step, envs map[string]string) error {
	t.Helper()

	dag := &ir.DAG{Name: "dry-run-check", WorkingDir: workDir}
	ctx := runctx.NewContext(context.Background(), dag, "", "")
	env := runtime.NewEnv(ctx, step)
	if len(envs) > 0 {
		env.Scope = env.Scope.WithEntries(envs, cmnvalue.EnvSourceStepEnv)
	}
	return runtime.CheckDryRunStep(runtime.WithEnv(ctx, env), step)
}
