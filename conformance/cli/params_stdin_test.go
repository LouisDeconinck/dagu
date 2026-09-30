// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cli_test

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

const (
	stdinDAGName   = "params_stdin"
	stdinDAGFile   = stdinDAGName + ".yaml"
	stdinSizeLimit = 1 << 20
)

type stdinRun struct {
	DAGRunID string `json:"dagRunId"`
	Status   string `json:"status"`
	Params   string `json:"params"`
}

func TestParamsStdinValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "Positional", input: "from-stdin\n", want: "from-stdin"},
		{name: "Named", input: "value=from-stdin\n", want: "from-stdin"},
		{name: "JSON", input: `{"value":"from-stdin"}`, want: "from-stdin"},
		{name: "Quoted", input: `"hello world"`, want: "hello world"},
		{name: "Spaced", input: `" hello world "`, want: " hello world "},
		{name: "EmptyValue", input: `""`, want: ""},
		{name: "EmptyInput", want: "default"},
		{name: "Whitespace", input: " \n\t\r\n", want: "default"},
	}
	for _, command := range []string{"start", "enqueue"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					dagu := harness.NewRunner(t)
					env := sharedEnv(t)
					result := dagu.RunWithStdin(env, strings.NewReader(tc.input),
						command, "--params-stdin", "--run-id="+stdinRunID(t), stdinDAGFile)
					result.ExpectExitCode(0)
					expectStdinRun(t, dagu, env, command, tc.want)
				})
			}
		})
	}
}

// Real file descriptors expose whether a command consumed its caller's input,
// which must remain available to shell loops and higher-precedence sources.
func TestParamsStdinPrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		flags []string
		dash  []string
		want  string
	}{
		{name: "Inherited", want: "default"},
		{name: "Disabled", flags: []string{"--params-stdin=false"}, want: "default"},
		{name: "Flag", flags: []string{"--params-stdin", "--params=value=flag"}, want: "flag"},
		{name: "EmptyFlag", flags: []string{"--params-stdin", "--params="}, want: "default"},
		{name: "Dash", flags: []string{"--params-stdin", "--params=value=flag"}, dash: []string{"--", "value=dash"}, want: "dash"},
		{name: "EmptyDash", flags: []string{"--params-stdin", "--params=value=flag"}, dash: []string{"--"}, want: "default"},
	}
	for _, command := range []string{"start", "enqueue"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					dagu := harness.NewRunner(t)
					env := sharedEnv(t)
					const input = "value=stdin\nnext-workflow\n"
					stdin, writer, err := os.Pipe()
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, stdin.Close()) })
					_, err = io.WriteString(writer, input)
					require.NoError(t, err)
					require.NoError(t, writer.Close())

					args := append([]string{command, "--run-id=" + stdinRunID(t)}, tc.flags...)
					args = append(args, stdinDAGFile)
					args = append(args, tc.dash...)
					dagu.RunWithStdin(env, stdin, args...).ExpectExitCode(0)
					expectStdinRun(t, dagu, env, command, tc.want)
					remaining, err := io.ReadAll(stdin)
					require.NoError(t, err)
					require.Equal(t, input, string(remaining))
				})
			}
		})
	}
}

// An inherited pipe need not reach EOF before an unselected command finishes.
func TestParamsStdinOpenPipe(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"start", "enqueue"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			dagu := harness.NewRunner(t)
			env := sharedEnv(t)
			stdin, writer, err := os.Pipe()
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, writer.Close())
				require.NoError(t, stdin.Close())
			})
			dagu.RunWithStdin(env, stdin, command, "--run-id="+stdinRunID(t), stdinDAGFile).ExpectExitCode(0)
			expectStdinRun(t, dagu, env, command, "default")
		})
	}
}

// The size limit applies before whitespace trimming and before run admission.
func TestParamsStdinLimit(t *testing.T) {
	t.Parallel()

	const params = "value=boundary"
	input := strings.Repeat(" ", stdinSizeLimit-len(params)) + params
	for _, command := range []string{"start", "enqueue"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			for _, size := range []int{stdinSizeLimit, stdinSizeLimit + 1} {
				name := "AtLimit"
				if size > stdinSizeLimit {
					name = "OverLimit"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					dagu := harness.NewRunner(t)
					env := sharedEnv(t)
					dagu.WriteFile("stdin.txt", input+strings.Repeat(" ", size-stdinSizeLimit))
					stdin, err := os.Open(dagu.ProjectPath("stdin.txt")) // #nosec G304 -- isolated test input.
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, stdin.Close()) })
					result := dagu.RunWithStdin(env, stdin, command, "--params-stdin", "--run-id="+stdinRunID(t), stdinDAGFile)
					if size == stdinSizeLimit {
						result.ExpectExitCode(0)
						expectStdinRun(t, dagu, env, command, "boundary")
						return
					}
					result.ExpectNonZeroExitCode()
					result.ExpectStderrContains("params from stdin exceed", "1048576 byte limit")
					require.Empty(t, stdinHistory(t, dagu, env))
					dagu.ExpectNoFile("params_stdin.out")
				})
			}
		})
	}
}

func TestParamsStdinFromRunID(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	env := sharedEnv(t)
	result := dagu.RunWithStdin(env, strings.NewReader("value=stdin"),
		"start", "--params-stdin", "--from-run-id=source", "--run-id="+stdinRunID(t), stdinDAGFile)
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("parameters cannot be provided when using --from-run-id")
	require.Empty(t, stdinHistory(t, dagu, env))
	dagu.ExpectNoFile("params_stdin.out")
}

func expectStdinRun(t *testing.T, dagu *harness.Runner, env []string, command, value string) {
	t.Helper()

	runs := stdinHistory(t, dagu, env)
	require.Len(t, runs, 1)
	status := "queued"
	if command == "start" {
		status = "succeeded"
		dagu.ExpectFileContent("params_stdin.out", value+"\n")
	}
	require.Equal(t, stdinRun{DAGRunID: stdinRunID(t), Status: status, Params: "value=" + value}, runs[0])
}

func stdinHistory(t *testing.T, dagu *harness.Runner, env []string) []stdinRun {
	t.Helper()

	result := dagu.RunWithEnv(env, "history", "--format=json", "--run-id="+stdinRunID(t), stdinDAGName)
	result.ExpectExitCode(0)
	var runs []stdinRun
	require.NoError(t, json.Unmarshal([]byte(result.Stdout()), &runs))
	return runs
}

// Run IDs distinguish process sockets even across isolated DAGU_HOME values.
func stdinRunID(t *testing.T) string {
	t.Helper()
	return strings.ReplaceAll(t.Name(), "/", "-")
}
