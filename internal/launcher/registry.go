// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package launcher

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"github.com/dagucloud/dagu/v2/internal/cmn/cmdutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
)

// processRegistryKey is the context key carrying a *ProcessRegistry.
type processRegistryKey struct{}

// ProcessRegistry tracks subprocesses launched through this package so a
// supervising process (server, scheduler, start-all) can forward shutdown
// signals to them when signal_handling.enable_propagation is enabled.
//
// Every launched command runs in its own process group (see
// cmdutil.SetupCommand), so signaling the command's PID reaches the whole
// process tree on Unix. On Windows the equivalent process-tree/job-object
// termination is used.
type ProcessRegistry struct {
	mu         sync.Mutex
	procs      map[*exec.Cmd]struct{}
	propagated bool
}

// NewProcessRegistry returns an empty ProcessRegistry.
func NewProcessRegistry() *ProcessRegistry {
	return &ProcessRegistry{procs: make(map[*exec.Cmd]struct{})}
}

// ContextWithProcessRegistry attaches reg to ctx so launcher entry points can
// register subprocesses they start.
func ContextWithProcessRegistry(ctx context.Context, reg *ProcessRegistry) context.Context {
	if reg == nil {
		return ctx
	}
	return context.WithValue(ctx, processRegistryKey{}, reg)
}

// ProcessRegistryFrom returns the ProcessRegistry attached to ctx, or nil when
// signal propagation is not enabled for this context.
func ProcessRegistryFrom(ctx context.Context) *ProcessRegistry {
	if ctx == nil {
		return nil
	}
	reg, _ := ctx.Value(processRegistryKey{}).(*ProcessRegistry)
	return reg
}

// PropagateSignal forwards sig to tracked subprocesses when a ProcessRegistry
// is attached to ctx (signal propagation enabled). It is a no-op otherwise.
func PropagateSignal(ctx context.Context, sig os.Signal) {
	if reg := ProcessRegistryFrom(ctx); reg != nil {
		reg.Propagate(ctx, sig)
	}
}

// track registers cmd for the duration of the returned func. Callers invoke the
// returned func once the command has exited.
func track(ctx context.Context, cmd *exec.Cmd) func() {
	reg := ProcessRegistryFrom(ctx)
	if reg == nil || cmd == nil {
		return func() {}
	}
	reg.mu.Lock()
	reg.procs[cmd] = struct{}{}
	reg.mu.Unlock()
	return func() {
		reg.mu.Lock()
		delete(reg.procs, cmd)
		reg.mu.Unlock()
	}
}

// Propagate forwards sig to the process group of every tracked subprocess. It
// runs at most once per registry; later calls are no-ops. Per-process errors
// are logged and do not stop propagation to the remaining processes.
func (r *ProcessRegistry) Propagate(ctx context.Context, sig os.Signal) {
	r.mu.Lock()
	if r.propagated {
		r.mu.Unlock()
		return
	}
	r.propagated = true
	procs := make([]*exec.Cmd, 0, len(r.procs))
	for cmd := range r.procs {
		procs = append(procs, cmd)
	}
	r.mu.Unlock()

	if len(procs) == 0 {
		return
	}
	intent := cmdutil.TerminationFromSignal(sig)
	logger.Info(ctx, "Propagating signal to running DAG processes",
		tag.Signal(intent.SignalName()),
		slog.Int("processes", len(procs)),
	)
	for _, cmd := range procs {
		// cmd.Process is immutable after Start; ProcessState is written by
		// Wait and must not be read here. Processes that exited but are not
		// yet reaped simply fail with ESRCH, which is logged below.
		if cmd == nil || cmd.Process == nil {
			continue
		}
		if err := cmdutil.TerminateProcessGroup(cmd, intent); err != nil {
			logger.Warn(ctx, "Failed to propagate signal to DAG process",
				tag.PID(cmd.Process.Pid),
				tag.Signal(intent.SignalName()),
				tag.Error(err),
			)
		}
	}
}
