// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dagucloud/dagu/v2/internal/cmn/stringutil"
	"github.com/dagucloud/dagu/v2/internal/spec"
)

// stdinHasParamsInput reports whether stdin is a pipe or a redirected file, so
// a run may read parameters from it. Terminals and other character devices
// (e.g. /dev/null) are never treated as params input, keeping non-piped runs
// free of stdin reads.
func stdinHasParamsInput() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice == 0
}

// readStdinParams returns all of stdin trimmed, for use as run params.
func readStdinParams() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("failed to read params from stdin: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func quoteStartDashArgs(args []string) []string {
	if isSingleJSONDashArg(args) {
		return args
	}
	return spec.QuoteRuntimeParams(args, nil)
}

func isSingleJSONDashArg(args []string) bool {
	if len(args) != 1 {
		return false
	}

	input := strings.TrimSpace(stringutil.RemoveQuotes(args[0]))
	if input == "" {
		return false
	}

	isObject := strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}")
	isArray := strings.HasPrefix(input, "[") && strings.HasSuffix(input, "]")
	if !isObject && !isArray {
		return false
	}
	return json.Valid([]byte(input))
}
