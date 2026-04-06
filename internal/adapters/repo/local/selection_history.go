package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports"
)

const (
	selectionHistoryFileMode = 0o600
	selectionHistoryDirMode  = 0o700
	selectionHistoryMaxItems = 256
	selectionHistoryTempFile = ".selection-history-*.json.tmp"
)

type SelectionHistory struct {
	path string
	mu   *sync.RWMutex
}

type selectionHistoryFile struct {
	Selections []selectionRecord `json:"selections"`
}

type selectionRecord struct {
	AccountID  string    `json:"account_id"`
	SelectedAt time.Time `json:"selected_at"`
}

var (
	selectionHistoryLockRegistryMu sync.Mutex
	selectionHistoryPathLockMap    = map[string]*sync.RWMutex{}
)

var _ ports.SelectionHistory = (*SelectionHistory)(nil)

func NewSelectionHistory(path string) *SelectionHistory {
	cleaned := filepath.Clean(path)
	if absPath, err := filepath.Abs(cleaned); err == nil {
		cleaned = absPath
	}

	return &SelectionHistory{path: cleaned, mu: selectionHistoryLockForPath(cleaned)}
}

func (h *SelectionHistory) RecordSelection(ctx context.Context, id domain.AccountID, selectedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	file, err := h.readFile()
	if err != nil {
		return err
	}

	file.Selections = append(file.Selections, selectionRecord{AccountID: string(id), SelectedAt: selectedAt.UTC()})
	sortSelectionRecords(file.Selections)
	if len(file.Selections) > selectionHistoryMaxItems {
		file.Selections = file.Selections[len(file.Selections)-selectionHistoryMaxItems:]
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := h.writeFile(file); err != nil {
		return err
	}

	return nil
}

func (h *SelectionHistory) RecentSelections(ctx context.Context, since time.Time) ([]domain.AccountID, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	file, err := h.readFile()
	if err != nil {
		return nil, err
	}

	recent := make([]selectionRecord, 0, len(file.Selections))
	for _, selection := range file.Selections {
		if selection.SelectedAt.Before(since) {
			continue
		}
		recent = append(recent, selection)
	}

	sortSelectionRecords(recent)

	ids := make([]domain.AccountID, 0, len(recent))
	for _, selection := range recent {
		ids = append(ids, domain.AccountID(selection.AccountID))
	}

	return ids, nil
}

func (h *SelectionHistory) readFile() (selectionHistoryFile, error) {
	data, err := os.ReadFile(h.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return selectionHistoryFile{}, nil
		}
		return selectionHistoryFile{}, fmt.Errorf("read selection history file: %w", err)
	}

	var file selectionHistoryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return selectionHistoryFile{}, fmt.Errorf("decode selection history file: %w", err)
	}

	return file, nil
}

func (h *SelectionHistory) writeFile(file selectionHistoryFile) error {
	if err := os.MkdirAll(filepath.Dir(h.path), selectionHistoryDirMode); err != nil {
		return fmt.Errorf("create selection history directory: %w", err)
	}

	data, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode selection history file: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(h.path), selectionHistoryTempFile)
	if err != nil {
		return fmt.Errorf("create temp selection history file: %w", err)
	}

	tempName := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp selection history file: %w", err)
	}

	if err := tempFile.Chmod(selectionHistoryFileMode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp selection history file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp selection history file: %w", err)
	}

	if err := os.Rename(tempName, h.path); err != nil {
		return fmt.Errorf("rename temp selection history file: %w", err)
	}

	cleanup = false
	return nil
}

func sortSelectionRecords(records []selectionRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].SelectedAt.Equal(records[j].SelectedAt) {
			return records[i].AccountID < records[j].AccountID
		}
		return records[i].SelectedAt.Before(records[j].SelectedAt)
	})
}

func selectionHistoryLockForPath(path string) *sync.RWMutex {
	selectionHistoryLockRegistryMu.Lock()
	defer selectionHistoryLockRegistryMu.Unlock()

	if mu, ok := selectionHistoryPathLockMap[path]; ok {
		return mu
	}

	mu := &sync.RWMutex{}
	selectionHistoryPathLockMap[path] = mu
	return mu
}
