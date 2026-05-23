package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errWriter struct {
	err error
}

func (w errWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func TestFetchAccountsConcurrentlyWithNoAccounts(t *testing.T) {
	summary, err := fetchAccountsConcurrently(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.Empty(t, summary.successes)
	assert.Empty(t, summary.failures)
}

func TestWriteFetchSummaryPlainReturnsWriterError(t *testing.T) {
	writeErr := errors.New("broken pipe")
	summary := fetchSummary{
		failures: []fetchResult{{
			accountID: domain.AccountID("acc-1"),
			err:       errors.New("boom"),
		}},
	}

	err := writeFetchSummaryPlain(errWriter{err: writeErr}, summary)
	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)
}
