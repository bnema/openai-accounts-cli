package local

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectionHistoryRecentSelectionsMissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()

	history := NewSelectionHistory(filepath.Join(t.TempDir(), "missing", "selection-history.json"))

	recent, err := history.RecentSelections(context.Background(), time.Date(2026, time.April, 6, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Empty(t, recent)
}

func TestSelectionHistoryRecordSelectionPersistsAccountIDsWithTimestamps(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "selection-history.json")
	history := NewSelectionHistory(path)
	recordedAt := time.Date(2026, time.April, 6, 10, 15, 0, 0, time.UTC)

	require.NoError(t, history.RecordSelection(context.Background(), domain.AccountID("acc-1"), recordedAt))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var payload struct {
		Selections []struct {
			AccountID  string    `json:"account_id"`
			SelectedAt time.Time `json:"selected_at"`
		} `json:"selections"`
	}
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Len(t, payload.Selections, 1)
	assert.Equal(t, "acc-1", payload.Selections[0].AccountID)
	assert.True(t, payload.Selections[0].SelectedAt.Equal(recordedAt))

	reloaded := NewSelectionHistory(path)
	recent, err := reloaded.RecentSelections(context.Background(), recordedAt.Add(-time.Minute))
	require.NoError(t, err)
	assert.Equal(t, []domain.AccountID{"acc-1"}, recent)
}

func TestSelectionHistoryRecentSelectionsFiltersWindowAndReturnsDeterministicOrder(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "selection-history.json")
	history := NewSelectionHistory(path)
	base := time.Date(2026, time.April, 6, 10, 0, 0, 0, time.UTC)

	require.NoError(t, history.RecordSelection(context.Background(), domain.AccountID("outside-window"), base.Add(-2*time.Hour)))
	require.NoError(t, history.RecordSelection(context.Background(), domain.AccountID("acc-2"), base.Add(2*time.Minute)))
	require.NoError(t, history.RecordSelection(context.Background(), domain.AccountID("acc-1"), base.Add(1*time.Minute)))
	require.NoError(t, history.RecordSelection(context.Background(), domain.AccountID("acc-3"), base.Add(3*time.Minute)))

	recent, err := history.RecentSelections(context.Background(), base)
	require.NoError(t, err)
	assert.Equal(t, []domain.AccountID{"acc-1", "acc-2", "acc-3"}, recent)
}

func TestSelectionHistoryRecentSelectionsReturnsDecodeErrorForMalformedJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "selection-history.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"selections":`), 0o600))

	history := NewSelectionHistory(path)

	_, err := history.RecentSelections(context.Background(), time.Date(2026, time.April, 6, 10, 0, 0, 0, time.UTC))
	require.Error(t, err)
	assert.ErrorContains(t, err, "decode selection history file")
}

func TestSelectionHistoryRecordSelectionTrimsBoundedRetention(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "selection-history.json")
	history := NewSelectionHistory(path)
	base := time.Date(2026, time.April, 6, 10, 0, 0, 0, time.UTC)

	for i := 0; i < selectionHistoryMaxItems+2; i++ {
		id := domain.AccountID("acc-" + time.Duration(i).String())
		require.NoError(t, history.RecordSelection(context.Background(), id, base.Add(time.Duration(i)*time.Minute)))
	}

	recent, err := history.RecentSelections(context.Background(), base.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, recent, selectionHistoryMaxItems)
	assert.Equal(t, domain.AccountID("acc-2ns"), recent[0])
	assert.Equal(t, domain.AccountID("acc-257ns"), recent[len(recent)-1])
}
