// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/ir"
)

const stepLogArchiveContentType = "application/zip"

type stepLogArchiveResponse struct {
	ctx      context.Context
	status   *ir.DAGRunStatus
	openLog  func(context.Context, string) (io.ReadCloser, error)
	filename string
}

func (r *stepLogArchiveResponse) VisitDownloadDAGRunStepLogsResponse(w http.ResponseWriter) error {
	return r.writeTo(w)
}

func (r *stepLogArchiveResponse) VisitDownloadSubDAGRunStepLogsResponse(w http.ResponseWriter) error {
	return r.writeTo(w)
}

func (r *stepLogArchiveResponse) writeTo(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", stepLogArchiveContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", r.filename))
	w.WriteHeader(http.StatusOK)
	if err := r.writeArchive(w); err != nil {
		logger.Error(r.ctx, "Failed to stream step log archive", tag.Error(err))
		// Headers are committed; abort instead of appending JSON or finalizing a partial ZIP.
		panic(http.ErrAbortHandler)
	}
	return nil
}

func (r *stepLogArchiveResponse) writeArchive(w io.Writer) error {
	archive := zip.NewWriter(w)
	for i, node := range r.status.NodesInRunOrder() {
		if node == nil {
			continue
		}
		directory := fmt.Sprintf("%03d-%s", i+1, fileutil.SafeName(node.Step.Name))
		for _, stream := range []struct{ name, path string }{
			{"stdout", node.Stdout}, {"stderr", node.Stderr},
		} {
			if err := r.ctx.Err(); err != nil {
				return err
			}
			if stream.path == "" {
				continue
			}
			reader, err := r.openLog(r.ctx, stream.path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			entry, err := archive.Create(directory + "/" + stream.name + ".log")
			if err == nil {
				_, err = io.Copy(entry, reader)
			}
			closeErr := reader.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	if err := r.ctx.Err(); err != nil {
		return err
	}
	return archive.Close()
}

func isStepLogDownload(r *http.Request, apiBasePath string) bool {
	if r.Method != http.MethodGet {
		return false
	}
	suffix, ok := strings.CutPrefix(r.URL.Path, strings.TrimRight(apiBasePath, "/")+"/dag-runs/")
	if !ok {
		return false
	}
	parts := strings.Split(suffix, "/")
	return (len(parts) == 5 && strings.Join(parts[2:], "/") == "steps/log/download") ||
		(len(parts) == 7 && parts[2] == "sub-dag-runs" && strings.Join(parts[4:], "/") == "steps/log/download")
}

func stepLogDownloadDeadline(apiBasePath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isStepLogDownload(r, apiBasePath) {
				if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
					logger.Error(r.Context(), "Failed to clear step log download deadline", tag.Error(err))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
