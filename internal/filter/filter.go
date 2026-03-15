package filter

import (
	"context"

	"github.com/nbitslabs/stenographer/internal/database/sqlc"
)

type Checker struct {
	queries *sqlc.Queries
	mode    string
}

func New(queries *sqlc.Queries, mode string) *Checker {
	return &Checker{
		queries: queries,
		mode:    mode,
	}
}

func (c *Checker) ShouldLog(ctx context.Context, chatID int64) (bool, error) {
	count, err := c.queries.IsChatFiltered(ctx, chatID)
	if err != nil {
		return false, err
	}
	isInList := count > 0

	if c.mode == "allowlist" {
		return isInList, nil
	}
	// blacklist mode (default): log if NOT in list
	return !isInList, nil
}
