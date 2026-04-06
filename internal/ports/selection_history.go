package ports

import (
	"context"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

type SelectionHistory interface {
	RecordSelection(ctx context.Context, id domain.AccountID, selectedAt time.Time) error
	RecentSelections(ctx context.Context, since time.Time) ([]domain.AccountID, error)
}
