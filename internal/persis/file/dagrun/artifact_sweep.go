// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dagrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/artifactpath"
	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/persis"
)

// minArtifactPruneAge bounds how fresh an artifact entry can be and still be
// reclaimed. A run's artifact directory is created before its record exists,
// so anything younger could still belong to a run that has not been recorded
// yet.
const minArtifactPruneAge = time.Hour

// PruneArtifacts implements persis.DAGRunStore. It removes artifact
// directories and index records that no surviving DAG run points to.
//
// Liveness is decided by name rather than by a status's ArchiveDir: a run
// directory name ends in the hash of its DAG-run ID, so a directory whose
// suffix no surviving run record produces is orphaned no matter how the
// record disappeared. Removal is bounded to entries older than
// req.OlderThan, clamped to minArtifactPruneAge, which keeps directories of
// runs that exist but have not written a record yet.
func (store *Store) PruneArtifacts(ctx context.Context, req persis.ArtifactPruneRequest) (*persis.ArtifactPruneResult, error) {
	root := strings.TrimSpace(req.Root)
	if root == "" {
		root = store.artifactDir
	}
	if root == "" {
		return nil, fmt.Errorf("artifact directory is not configured")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("invalid artifact root %q: %w", root, err)
	}
	if filepath.Dir(abs) == abs {
		return nil, fmt.Errorf("refusing to sweep filesystem root %q", abs)
	}

	// An enumeration failure aborts the sweep: a run missed here would look
	// orphaned and its artifacts would be reclaimed while still live.
	live, err := store.liveArtifactNames(ctx)
	if err != nil {
		return nil, err
	}

	cutoff := req.OlderThan.Time
	if cutoff.IsZero() {
		cutoff = time.Now()
	}
	if floor := time.Now().Add(-minArtifactPruneAge); cutoff.After(floor) {
		cutoff = floor
	}

	s := &artifactSweep{
		root:    filepath.Clean(root),
		cutoff:  cutoff,
		dryRun:  req.DryRun,
		live:    live,
		emptied: map[string]struct{}{},
	}
	if err := s.run(ctx); err != nil {
		return nil, err
	}
	return &s.result, nil
}

// artifactSweep carries the liveness set and policy for one sweep of an
// artifact root.
type artifactSweep struct {
	root    string
	cutoff  time.Time
	dryRun  bool
	live    map[string]struct{}
	result  persis.ArtifactPruneResult
	emptied map[string]struct{}
}

func (s *artifactSweep) run(ctx context.Context) error {
	if err := s.sweepDir(ctx, s.root); err != nil {
		return err
	}
	if s.dryRun {
		return nil
	}

	// Drop directories that a removal emptied, deepest first so a parent is
	// seen after its children. The root itself is never removed.
	dirs := make([]string, 0, len(s.emptied))
	for dir := range s.emptied {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		// A directory that still holds entries fails to remove and stays.
		_ = fileutil.Remove(dir)
	}
	return nil
}

func (s *artifactSweep) sweepDir(ctx context.Context, dir string) error {
	entries, err := fileutil.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		if dir == s.root {
			return fmt.Errorf("failed to read artifact root %s: %w", dir, err)
		}
		// An unreadable directory is left alone; skipping it can only leave
		// entries behind, never remove a live one.
		logger.Error(ctx, "Failed to read artifact directory", tag.Error(err), tag.Dir(dir))
		return nil
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		path := filepath.Join(dir, name)

		if entry.IsDir() {
			if s.sweepRunDir(ctx, path, name) {
				continue
			}
			if err := s.sweepDir(ctx, path); err != nil {
				return err
			}
			continue
		}

		// An index record lives beside the run directory it describes, so a
		// .meta name is a removal candidate under the same rule.
		if artifactpath.IsMetaName(name) {
			s.sweepRecord(ctx, path, artifactpath.TrimMetaSuffix(name))
		}
	}
	return nil
}

// sweepRunDir evaluates a subdirectory and reports whether the walk must not
// descend into it. Both layouts are terminal: what sits beneath a run
// directory is run content, not entries the sweep manages.
func (s *artifactSweep) sweepRunDir(ctx context.Context, path, name string) bool {
	// The pre-date layout kept run directories as dag-run_<ts>_<id> under a
	// per-DAG directory; the name itself carries the timestamp and run ID.
	if matches := reDAGRunDir.FindStringSubmatch(name); len(matches) == 3 {
		at, err := parseDAGRunTimestamp(matches[1])
		if err == nil {
			s.maybeRemove(ctx, path, matches[2], at, true)
		}
		return true
	}

	parsed, ok := artifactpath.ParseRunDirName(name)
	if !ok {
		return false
	}
	day, ok := runDirDay(s.root, path)
	if !ok {
		return true
	}
	s.maybeRemove(ctx, path, parsed.Suffix, runDirTimestamp(day, parsed.TimeOfDay), true)
	return true
}

// sweepRecord evaluates an index sidecar by the name of the run directory it
// describes. A record whose directory lives outside the sweep root — because
// artifacts.dir relocated it — still lands in the tree and is reclaimed by
// the same rule.
func (s *artifactSweep) sweepRecord(ctx context.Context, path, name string) {
	parsed, ok := artifactpath.ParseRunDirName(name)
	if !ok {
		return
	}
	day, ok := runDirDay(s.root, path)
	if !ok {
		return
	}
	s.maybeRemove(ctx, path, parsed.Suffix, runDirTimestamp(day, parsed.TimeOfDay), false)
}

// maybeRemove removes path when its key is claimed by no surviving run and
// its timestamp predates the cutoff.
func (s *artifactSweep) maybeRemove(ctx context.Context, path, key string, at time.Time, dir bool) {
	if _, ok := s.live[key]; ok || at.IsZero() || !at.Before(s.cutoff) {
		return
	}
	if s.dryRun {
		s.report(path, dir)
		return
	}
	var err error
	if dir {
		err = fileutil.RemoveAll(path)
	} else {
		err = fileutil.Remove(path)
	}
	if err != nil {
		logger.Error(ctx, "Failed to remove orphaned artifact entry",
			tag.Error(err), tag.Dir(path))
		return
	}
	s.report(path, dir)
	s.markAncestors(path)
}

func (s *artifactSweep) report(path string, dir bool) {
	if dir {
		s.result.Dirs = append(s.result.Dirs, path)
		return
	}
	s.result.Records = append(s.result.Records, path)
}

// markAncestors records the directories between path and the root so that
// date partitions a removal emptied are pruned once the sweep finishes.
func (s *artifactSweep) markAncestors(path string) {
	for parent := filepath.Dir(path); parent != s.root && parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
		s.emptied[parent] = struct{}{}
	}
}

// runDirDay returns the "YYYY/MM/DD" key a path sits under, or false when its
// ancestors are not a date partition. The ancestors are taken from the path
// rather than assumed, so a relocated tree nested inside the root still
// resolves.
func runDirDay(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return "", false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 {
		return "", false
	}
	year, month, day := parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1]
	if !reYear.MatchString(year) || !reMonth.MatchString(month) || !reDay.MatchString(day) {
		return "", false
	}
	return year + "/" + month + "/" + day, true
}

// runDirTimestamp rebuilds when a partitioned run directory was created from
// its day and the time of day in its name.
func runDirTimestamp(day, timeOfDay string) time.Time {
	at, err := time.ParseInLocation("2006/01/02150405", day+timeOfDay, time.UTC)
	if err != nil {
		return time.Time{}
	}
	return at
}

// liveArtifactNames collects the names that keep an artifact directory alive:
// every run ID in the dag-runs tree, plus the directory name suffix derived
// from it. Directory names alone carry the ID, so no status file is read.
func (store *Store) liveArtifactNames(ctx context.Context) (map[string]struct{}, error) {
	roots, err := store.listRoot(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list DAG data roots: %w", err)
	}
	live := map[string]struct{}{}
	for _, root := range roots {
		if err := collectRunIDs(ctx, root.dagRunsDir, live); err != nil {
			return nil, err
		}
	}
	return live, nil
}

func collectRunIDs(ctx context.Context, dagRunsDir string, live map[string]struct{}) error {
	years, err := listDirsSorted(dagRunsDir, false, reYear)
	if err != nil {
		return fmt.Errorf("failed to list run years under %s: %w", dagRunsDir, err)
	}
	for _, year := range years {
		if err := ctx.Err(); err != nil {
			return err
		}
		months, err := listDirsSorted(filepath.Join(dagRunsDir, year), false, reMonth)
		if err != nil {
			return fmt.Errorf("failed to list run months under %s: %w", filepath.Join(dagRunsDir, year), err)
		}
		for _, month := range months {
			monthPath := filepath.Join(dagRunsDir, year, month)
			days, err := listDirsSorted(monthPath, false, reDay)
			if err != nil {
				return fmt.Errorf("failed to list run days under %s: %w", monthPath, err)
			}
			for _, day := range days {
				if err := collectDayRunIDs(ctx, filepath.Join(monthPath, day), live); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func collectDayRunIDs(ctx context.Context, dayPath string, live map[string]struct{}) error {
	entries, err := fileutil.ReadDir(dayPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read run directory %s: %w", dayPath, err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			continue
		}
		matches := reDAGRunDir.FindStringSubmatch(entry.Name())
		if len(matches) != 3 {
			continue
		}
		markLive(matches[2], live)
		if err := collectSubRunIDs(ctx, filepath.Join(dayPath, entry.Name()), live); err != nil {
			return err
		}
	}
	return nil
}

// collectSubRunIDs marks the run IDs nested under a run directory's sub
// dag-run directories. A sub dag-run can hold children of its own, so this
// recurses.
func collectSubRunIDs(ctx context.Context, runDir string, live map[string]struct{}) error {
	for _, dirName := range []string{SubDAGRunsDir, LegacySubDAGRunsDir} {
		subDir := filepath.Join(runDir, dirName)
		entries, err := fileutil.ReadDir(subDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to read sub dag-run directory %s: %w", subDir, err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !entry.IsDir() {
				continue
			}
			dagRunID, ok := subDAGRunIDFromDir(dirName, entry.Name())
			if !ok {
				continue
			}
			markLive(dagRunID, live)
			if err := collectSubRunIDs(ctx, filepath.Join(subDir, entry.Name()), live); err != nil {
				return err
			}
		}
	}
	return nil
}

// markLive records both names a run's artifact directory can be keyed by: the
// run ID verbatim for the pre-date layout and the derived suffix for the
// partitioned layout.
func markLive(dagRunID string, live map[string]struct{}) {
	live[dagRunID] = struct{}{}
	live[artifactpath.RunSuffix(dagRunID)] = struct{}{}
}
