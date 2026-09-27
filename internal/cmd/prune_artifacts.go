// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/v2/internal/persis"
	"github.com/spf13/cobra"
)

// PruneArtifacts creates and returns a cobra command for reclaiming orphaned
// artifact directories.
func PruneArtifacts() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "prune-artifacts [flags]",
			Short: "Remove artifact directories no surviving DAG run points to",
			Long: `Remove artifact directories and index records that no surviving DAG run
points to.

Run history is normally what removes artifacts, so directories are orphaned
when a run record is deleted by another route or the artifact root moved.
Liveness is decided by name: a directory or record is removed only when no
run in the current history tree could still claim it, and only when it is
older than --older-than. A minimum age of 1h is always enforced because a
run's artifact directory exists before its record does.

Flags:
  -t, --older-than   Only remove entries older than a duration (e.g. 10d,
                     24h, 1w; default: 24h, minimum enforced: 1h)
      --root         Artifact root to prune (default: configured
                     paths.artifact_dir). Use this to reclaim a tree left at
                     a previous data directory
      --dry-run      Preview what would be removed without removing
  -y, --yes          Skip confirmation prompt

Examples:
  dagu prune-artifacts --dry-run                  # Preview orphans under the configured root
  dagu prune-artifacts -t 7d -y                   # Remove orphans older than 7 days
  dagu prune-artifacts --root /old/data/artifacts # Reclaim a tree left at an old data_dir
`,
			Args: cobra.NoArgs,
		},
		pruneArtifactsFlags,
		runPruneArtifacts,
	)
}

var pruneArtifactsFlags = []commandLineFlag{
	pruneArtifactsOlderThanFlag,
	pruneArtifactsRootFlag,
	dryRunFlag,
	yesFlag,
}

func runPruneArtifacts(ctx *Context, _ []string) error {
	dryRun, _ := ctx.Command.Flags().GetBool("dry-run")
	skipConfirm, _ := ctx.Command.Flags().GetBool("yes")

	olderThan, err := ctx.StringParam("older-than")
	if err != nil {
		return fmt.Errorf("failed to get older-than: %w", err)
	}
	dur, err := parseRelativeDuration(olderThan)
	if err != nil {
		return fmt.Errorf("invalid --older-than value %q: %w. Valid formats: 7d, 24h, 1w", olderThan, err)
	}

	root, err := ctx.StringParam("root")
	if err != nil {
		return fmt.Errorf("failed to get root: %w", err)
	}
	root = fileutil.ResolvePathOrBlank(root)

	displayRoot := root
	if displayRoot == "" {
		displayRoot = ctx.Config.Paths.ArtifactDir
	}

	if !dryRun && !skipConfirm && !ctx.Quiet {
		fmt.Printf("This will delete orphaned artifact entries older than %s under %s.\n", dur, displayRoot)
		fmt.Println("Entries a surviving DAG run references are kept.")
		if !confirmAction("Continue?") {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	result, err := ctx.Persistence.DAGRunRepository.PruneArtifacts(ctx, persis.ArtifactPruneRequest{
		Root:      root,
		OlderThan: persis.NewUTC(time.Now().UTC().Add(-dur)),
		DryRun:    dryRun,
	})
	if err != nil {
		return fmt.Errorf("failed to prune artifacts: %w", err)
	}

	total := len(result.Dirs) + len(result.Records)
	if dryRun {
		if total == 0 {
			fmt.Printf("Dry run: no orphaned artifacts to remove under %s\n", displayRoot)
			return nil
		}
		fmt.Printf("Dry run: would remove %d artifact director(ies) and %d index record(s) under %s:\n",
			len(result.Dirs), len(result.Records), displayRoot)
		for _, dir := range result.Dirs {
			fmt.Printf("  - %s\n", dir)
		}
		for _, record := range result.Records {
			fmt.Printf("  - %s\n", record)
		}
		return nil
	}

	if !ctx.Quiet {
		fmt.Printf("Removed %d artifact director(ies) and %d index record(s) under %s\n",
			len(result.Dirs), len(result.Records), displayRoot)
	}
	return nil
}
