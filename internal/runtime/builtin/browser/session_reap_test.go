// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/browserhost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reap runs a session's watchdog until it returns, and returns its error.
func (ts *testSessions) reap(id string) <-chan error {
	done := make(chan error, 1)
	go func() { done <- ts.sessions.Reap(ts.context(), id) }()
	return done
}

func waitReap(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the watchdog did not return")
	}
}

// The watchdog closes a session's browser once the session is idle past
// its timeout, keeping its history, and waits out a deadline a command
// renewed.
func TestSessionWatchdogClosesAnIdleSession(t *testing.T) {
	t.Parallel()

	ts := newTestSessions(t, pageModel(nil))
	ts.sessions.now = time.Now
	opened := ts.open(SessionOptions{URL: "https://portal.example.com/login", IdleTimeout: 600 * time.Millisecond})
	done := ts.reap(opened.ID)

	time.Sleep(300 * time.Millisecond)
	_, err := ts.do(opened.ID, `{"goto": "https://portal.example.com/help"}`)
	require.NoError(t, err)
	renewed := ts.record(opened.ID).Deadline
	time.Sleep(400 * time.Millisecond)
	assert.Equal(t, browserhost.StateInteractive, ts.record(opened.ID).State, "the command renewed the session past the first deadline")

	waitReap(t, done)
	record := ts.record(opened.ID)
	assert.Equal(t, browserhost.StateEnded, record.State)
	assert.True(t, time.Now().After(renewed))
	assert.Len(t, ts.state(opened.ID).Ops, 1, "the history is kept")
}

// The watchdog of a session closed by hand has nothing left to do.
func TestSessionWatchdogEndsWithItsSession(t *testing.T) {
	t.Parallel()

	ts := newTestSessions(t, pageModel(nil))
	ts.sessions.now = time.Now
	opened := ts.open(SessionOptions{URL: "https://portal.example.com/login", IdleTimeout: 300 * time.Millisecond})
	require.NoError(t, ts.sessions.Close(ts.context(), CloseRequest{ID: opened.ID}))
	waitReap(t, ts.reap(opened.ID))
}
