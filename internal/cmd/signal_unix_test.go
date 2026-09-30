// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package cmd_test

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/test"
	"github.com/stretchr/testify/require"
)

// Real command processes exercise shared registry ownership, signal-aware
// service contexts, and persistence after the supervisor has exited.
func TestSignalPropagation(t *testing.T) {
	for _, commandName := range []string{"server", "scheduler", "start-all"} {
		for _, shutdownSignal := range []os.Signal{syscall.SIGTERM, syscall.SIGINT} {
			t.Run(commandName+"/"+shutdownSignal.String(), func(t *testing.T) {
				th := test.SetupCommand(t, test.WithBuiltExecutable())
				ready := filepath.Join(t.TempDir(), "ready")
				stopped := filepath.Join(t.TempDir(), "signal")
				cleaned := filepath.Join(t.TempDir(), "cleaned")
				dag := th.DAG(t, fmt.Sprintf(`
type: graph
max_clean_up_time_sec: 5
steps:
  - id: probe
    shell: /bin/sh
    script: |
      trap 'printf TERM > %s; sleep 0.1; printf done > %s; exit 0' TERM
      trap 'printf INT > %s; sleep 0.1; printf done > %s; exit 0' INT
      printf ready > %s
      while :; do sleep 0.05; done
`, test.PosixQuote(stopped), test.PosixQuote(cleaned), test.PosixQuote(stopped), test.PosixQuote(cleaned), test.PosixQuote(ready)))
				args := []string{commandName}
				port := ""
				startupLog := "Scheduler started"
				if commandName != "scheduler" {
					port = findPort(t)
					args = append(args, "--host=127.0.0.1", "--port="+port)
					startupLog = "Server is starting"
				}
				command := exec.Command(th.Config.Paths.Executable, test.WithConfigFlag(args, th.Config)...) //nolint:gosec // Test executes the repository binary.
				command.Env = append(th.ChildEnv, "DAGU_SIGNAL_PROPAGATION=true", "GOMAXPROCS=1")
				command.Stdout = th.LoggingOutput
				command.Stderr = th.LoggingOutput
				require.NoError(t, command.Start())
				waitCh := make(chan error, 1)
				go func() { waitCh <- command.Wait() }()
				exited := false
				defer func() {
					if !exited {
						terminateTestCommand(command, waitCh)
					}
				}()
				require.Eventually(t, func() bool {
					return strings.Contains(th.LoggingOutput.String(), startupLog)
				}, commandLogWaitTimeout(), 20*time.Millisecond, "output: %s", th.LoggingOutput.String())
				if commandName == "server" {
					client := &http.Client{Timeout: time.Second}
					baseURL := "http://127.0.0.1:" + port + "/api/v1"
					require.Eventually(t, func() bool {
						resp, err := client.Get(baseURL + "/health")
						if err != nil {
							return false
						}
						_ = resp.Body.Close()
						return resp.StatusCode == http.StatusOK
					}, commandLogWaitTimeout(), 20*time.Millisecond)
					fileName := strings.TrimSuffix(filepath.Base(dag.Location), ".yaml")
					resp, err := client.Post(baseURL+"/dags/"+url.PathEscape(fileName)+"/start", "application/json", strings.NewReader("{}"))
					require.NoError(t, err)
					_ = resp.Body.Close()
					require.Equal(t, http.StatusOK, resp.StatusCode)
				} else {
					enqueue := exec.Command(th.Config.Paths.Executable, test.WithConfigFlag([]string{"enqueue", dag.Location}, th.Config)...) //nolint:gosec // Test executes the repository binary.
					enqueue.Env = command.Env
					output, err := enqueue.CombinedOutput()
					require.NoError(t, err, "output: %s", output)
				}
				require.Eventually(t, func() bool {
					_, err := os.Stat(ready)
					return err == nil
				}, commandLogWaitTimeout(), 20*time.Millisecond, "output: %s", th.LoggingOutput.String())
				require.NoError(t, command.Process.Signal(shutdownSignal))
				select {
				case err := <-waitCh:
					exited = true
					require.NoError(t, err, "output: %s", th.LoggingOutput.String())
				case <-time.After(commandLogWaitTimeout()):
					t.Fatal("supervisor did not shut down")
				}
				require.Eventually(t, func() bool {
					_, err := os.Stat(cleaned)
					return err == nil
				}, commandLogWaitTimeout(), 20*time.Millisecond, "step cleanup did not finish")
				data, err := os.ReadFile(stopped)
				require.NoError(t, err)
				want := "TERM"
				if shutdownSignal == syscall.SIGINT {
					want = "INT"
				}
				require.Equal(t, want, string(data))
				dag.AssertLatestStatus(t, ir.Aborted)
			})
		}
	}
}
