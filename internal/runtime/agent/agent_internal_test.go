// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/cmn/mailer/oauthconfig"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/runtime"
	"github.com/dagucloud/dagu/v2/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailerConfigFromSMTP(t *testing.T) {
	t.Parallel()

	config, err := mailerConfigFromSMTP(&ir.SMTPConfig{
		Username: "sender@example.com",
		OAuth: &oauthconfig.Config{
			Provider: oauthconfig.ProviderMicrosoft, TenantID: "tenant",
			ClientID: "client", ClientSecret: "secret",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "smtp.office365.com", config.Host)
	assert.Equal(t, "587", config.Port)
	assert.Equal(t, "sender@example.com", config.Username)
	assert.NotNil(t, config.Token)
}

func TestErrorString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "NilError",
			err:      nil,
			expected: "",
		},
		{
			name:     "SimpleError",
			err:      errors.New("test error"),
			expected: "test error",
		},
		{
			name:     "WrappedError",
			err:      errors.New("outer: inner error"),
			expected: "outer: inner error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errorString(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPanicToError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		panicObj    any
		expectedMsg string
	}{
		{
			name:        "WithError",
			panicObj:    errors.New("panic error"),
			expectedMsg: "panic error",
		},
		{
			name:        "WithString",
			panicObj:    "string panic",
			expectedMsg: "panic: string panic",
		},
		{
			name:        "WithInt",
			panicObj:    42,
			expectedMsg: "panic: 42",
		},
		{
			name:        "WithNil",
			panicObj:    nil,
			expectedMsg: "panic: <nil>",
		},
		{
			name:        "WithStruct",
			panicObj:    struct{ msg string }{msg: "test"},
			expectedMsg: "panic: {test}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := panicToError(tt.panicObj)
			assert.Equal(t, tt.expectedMsg, result.Error())
		})
	}
}

func TestEvalHostConfigObjectResolvesDAGParams(t *testing.T) {
	t.Parallel()

	// DAG-level ssh fields resolve Dagu-owned references like ${params.*} in
	// addition to unqualified environment syntax such as ${fqdn}.
	yaml := `
params:
  - name: fqdn
    type: string
  - name: bastion_host
    type: string
ssh:
  user: root
  host: ${params.fqdn}
  key: /keys/${fqdn}/id_rsa
  bastion:
    host: ${params.bastion_host}
    user: root
steps:
  - name: ok
    run: "true"
`
	dag, err := spec.LoadYAML(context.Background(), []byte(yaml),
		spec.WithParams("fqdn=node.internal bastion_host=bastion.internal"),
	)
	require.NoError(t, err)
	require.NotNil(t, dag.SSH)

	ctx := runtime.NewContext(context.Background(), dag, "test-run",
		filepath.Join(t.TempDir(), "run.log"),
		runtime.WithParams([]string{"fqdn=node.internal", "bastion_host=bastion.internal"}),
	)

	evaluated, err := evalHostConfigObject(ctx, *dag.SSH, runtime.GetEnv(ctx).UserEnvsMap(), "ssh")
	require.NoError(t, err)
	assert.Equal(t, "node.internal", evaluated.Host)
	assert.Equal(t, "/keys/node.internal/id_rsa", evaluated.Key)
	require.NotNil(t, evaluated.Bastion)
	assert.Equal(t, "bastion.internal", evaluated.Bastion.Host)
}

func TestEvalHostConfigObjectPreservesUnknownParams(t *testing.T) {
	t.Parallel()

	yaml := `
params:
  - name: fqdn
    type: string
ssh:
  user: root
  host: ${params.missing}
steps:
  - name: ok
    run: "true"
`
	dag, err := spec.LoadYAML(context.Background(), []byte(yaml),
		spec.WithParams("fqdn=node.internal"),
	)
	require.NoError(t, err)
	require.NotNil(t, dag.SSH)

	ctx := runtime.NewContext(context.Background(), dag, "test-run",
		filepath.Join(t.TempDir(), "run.log"),
		runtime.WithParams([]string{"fqdn=node.internal"}),
	)

	// An unresolvable params reference stays literal, matching the
	// warning-only semantics of other value-resolved fields.
	evaluated, err := evalHostConfigObject(ctx, *dag.SSH, runtime.GetEnv(ctx).UserEnvsMap(), "ssh")
	require.NoError(t, err)
	assert.Equal(t, "${params.missing}", evaluated.Host)
}
