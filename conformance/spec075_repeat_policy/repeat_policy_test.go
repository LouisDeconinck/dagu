// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec075_repeat_policy_test

import (
	"runtime"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

// A repeat condition that cannot be evaluated is a step failure, not a loop
// answer: a while loop stops and fails, and an until loop fails instead of
// repeating forever.
func TestRuntimeRepeatConditionEvaluationErrorUnix(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use POSIX shell snippets")
	}

	cases := []string{
		"until_eval_error_fails.yaml",
		"while_eval_error_fails.yaml",
	}
	for _, file := range cases {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run("start", file)
			result.ExpectExitCode(1)
			dagu.ExpectFileContent("ticks.txt", "tick\n")
		})
	}
}

func TestRuntimeRepeatDecisionUnix(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use POSIX shell snippets")
	}

	cases := []struct {
		name  string
		file  string
		ticks string
	}{
		{
			name:  "until not-met repeats until the limit",
			file:  "until_not_met_respects_limit.yaml",
			ticks: "tick\ntick\ntick\n",
		},
		{
			name:  "until met stops after the first attempt",
			file:  "until_met_stops.yaml",
			ticks: "tick\n",
		},
		{
			name:  "while not-met stops after the first attempt",
			file:  "while_not_met_stops.yaml",
			ticks: "tick\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run("start", tc.file)
			result.ExpectExitCode(0)
			dagu.ExpectFileContent("ticks.txt", tc.ticks)
		})
	}
}
