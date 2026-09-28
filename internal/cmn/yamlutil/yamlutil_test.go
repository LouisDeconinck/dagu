// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package yamlutil_test

import (
	"strings"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/cmn/yamlutil"
	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClearEmptyDocumentSeparators(t *testing.T) {
	t.Run("NoEmptyDocument", func(t *testing.T) {
		data := []byte("a: 1\n---\nb: 2\n")
		assert.Equal(t, data, yamlutil.ClearEmptyDocumentSeparators(data))
	})

	t.Run("EmptyDocumentBetweenDocs", func(t *testing.T) {
		data := []byte("a: 1\n---\n---\nb: 2\n")
		got := yamlutil.ClearEmptyDocumentSeparators(data)
		// The empty document's marker is blanked; both real documents parse.
		file, err := parser.ParseBytes(got, 0)
		require.NoError(t, err)
		require.Len(t, file.Docs, 2)
	})

	t.Run("EmptyDocumentWithComment", func(t *testing.T) {
		data := []byte("a: 1\n--- # nothing here\n---\nb: 2\n")
		file, err := parser.ParseBytes(yamlutil.ClearEmptyDocumentSeparators(data), 0)
		require.NoError(t, err)
		require.Len(t, file.Docs, 2)
	})

	t.Run("LeadingEmptyDocuments", func(t *testing.T) {
		data := []byte("---\n---\na: 1\n")
		file, err := parser.ParseBytes(yamlutil.ClearEmptyDocumentSeparators(data), 0)
		require.NoError(t, err)
		require.Len(t, file.Docs, 1)
	})

	t.Run("MarkerInsideLiteralBlockUntouched", func(t *testing.T) {
		data := []byte("a: |\n  ---\n  text\n---\nb: 2\n")
		got := yamlutil.ClearEmptyDocumentSeparators(data)
		assert.Equal(t, data, got)
		file, err := parser.ParseBytes(got, 0)
		require.NoError(t, err)
		require.Len(t, file.Docs, 2)
	})

	t.Run("LineCountPreserved", func(t *testing.T) {
		data := []byte("a: 1\n---\n---\nb: 2\n")
		got := yamlutil.ClearEmptyDocumentSeparators(data)
		assert.Equal(t,
			strings.Count(string(data), "\n"),
			strings.Count(string(got), "\n"))
	})

	t.Run("ParserDropsDocWithoutGuard", func(t *testing.T) {
		// Pins the parser quirk this package works around: without the guard
		// the document after an empty document never reaches the caller.
		data := []byte("a: 1\n---\n---\nb: 2\n")

		raw, err := parser.ParseBytes(data, 0)
		require.NoError(t, err)
		require.Len(t, raw.Docs, 2)
		require.Nil(t, raw.Docs[1].Body)

		guarded, err := parser.ParseBytes(yamlutil.ClearEmptyDocumentSeparators(data), 0)
		require.NoError(t, err)
		require.Len(t, guarded.Docs, 2)
		assert.Equal(t, "b: 2", guarded.Docs[1].Body.String())
	})
}
