// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmdutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LookupEnv returns the value of the first "key=value" entry in envs whose
// key matches key. The key comparison is case-insensitive on Windows.
func LookupEnv(envs []string, key string) (string, bool) {
	for _, entry := range envs {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if runtime.GOOS == "windows" {
			if strings.EqualFold(name, key) {
				return value, true
			}
			continue
		}
		if name == key {
			return value, true
		}
	}
	return "", false
}

// IsExecutableFile reports whether path names an existing regular file with
// executable permission. On Windows any existing regular file counts.
func IsExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

// LookPathInEnv resolves name to an executable file using the PATH entry in
// envs, honoring PATHEXT on Windows. When envs carries no PATH entry it falls
// back to FindExecutable. Names containing a path separator are checked as
// file paths.
func LookPathInEnv(name string, envs []string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		if IsExecutableFile(name) {
			return name, nil
		}
		return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	if pathEnv, ok := LookupEnv(envs, "PATH"); ok {
		pathextEnv, _ := LookupEnv(envs, "PATHEXT")
		return lookPathInPATH(name, pathEnv, pathextEnv)
	}
	if resolved, ok := FindExecutable(name); ok {
		return resolved, nil
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func lookPathInPATH(command, pathEnv, pathextEnv string) (string, error) {
	var lastErr error
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		for _, candidate := range pathCandidates(filepath.Join(dir, command), pathextEnv) {
			if IsExecutableFile(candidate) {
				return candidate, nil
			}
			if _, err := os.Stat(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
				lastErr = err
			}
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", &exec.Error{Name: command, Err: exec.ErrNotFound}
}

func pathCandidates(candidate, pathextEnv string) []string {
	if runtime.GOOS != "windows" || filepath.Ext(candidate) != "" {
		return []string{candidate}
	}

	pathext := pathextEnv
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	exts := strings.Split(pathext, ";")
	candidates := make([]string, 0, len(exts)+1)
	candidates = append(candidates, candidate)
	for _, ext := range exts {
		if ext == "" {
			continue
		}
		candidates = append(candidates, candidate+ext)
	}
	return candidates
}

// FindExecutable resolves cmd from PATH first, then falls back to common
// Windows compatibility locations for Git-provided Unix tooling.
func FindExecutable(cmd string) (string, bool) {
	if cmd == "" {
		return "", false
	}
	if path, err := exec.LookPath(cmd); err == nil {
		return path, true
	}
	if runtime.GOOS != "windows" {
		return "", false
	}
	if path := findWindowsCompatExecutable(cmd); path != "" {
		return path, true
	}
	return "", false
}

// ResolveExecutable returns the best-effort resolved executable path for cmd.
// If no compatibility path is found, the original value is returned unchanged.
func ResolveExecutable(cmd string) string {
	if runtime.GOOS != "windows" {
		return cmd
	}
	if path, ok := FindExecutable(cmd); ok {
		return path
	}
	return cmd
}

func findWindowsCompatExecutable(cmd string) string {
	name := strings.ToLower(filepath.Base(strings.ReplaceAll(cmd, "\\", "/")))
	name = strings.TrimSuffix(name, ".exe")

	var candidates []string
	switch name {
	case "bash":
		candidates = windowsGitCandidates("bash.exe")
	case "sh":
		candidates = windowsGitCandidates("sh.exe")
	case "env":
		candidates = windowsGitCandidates("env.exe")
	default:
		return ""
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate
		}
	}
	return ""
}

func windowsGitCandidates(exe string) []string {
	var roots []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			roots = append(roots, value)
		}
	}

	var candidates []string
	for _, root := range roots {
		switch filepath.Base(root) {
		case "Programs":
			candidates = append(candidates,
				filepath.Join(root, "Git", "bin", exe),
				filepath.Join(root, "Git", "usr", "bin", exe),
			)
		default:
			candidates = append(candidates,
				filepath.Join(root, "Git", "bin", exe),
				filepath.Join(root, "Git", "usr", "bin", exe),
				filepath.Join(root, "Programs", "Git", "bin", exe),
				filepath.Join(root, "Programs", "Git", "usr", "bin", exe),
			)
		}
	}

	return candidates
}
