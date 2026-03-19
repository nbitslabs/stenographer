package filter

import (
	"context"

	"github.com/nbitslabs/stenographer/internal/database/sqlc"
)

// Checker determines whether a message from a given chat should be logged.
type Checker struct {
	queries *sqlc.Queries
	mode    string // "default" or "allowlist_only"
}

// New creates a filter checker.
//
// Modes:
//   - "default": channels require whitelisting; groups and 1:1 chats are
//     logged unless blacklisted.
//   - "allowlist_only": all chats must be explicitly whitelisted.
//   - Any other value (including the legacy "blacklist") is treated as "default".
func New(queries *sqlc.Queries, mode string) *Checker {
	if mode != "default" && mode != "allowlist_only" {
		mode = "default"
	}
	return &Checker{
		queries: queries,
		mode:    mode,
	}
}

// ShouldLog returns true if messages from this chat should be stored.
func (c *Checker) ShouldLog(ctx context.Context, chatID int64, chatType string) (bool, error) {
	if c.mode == "allowlist_only" {
		return c.isWhitelisted(ctx, chatID, chatType)
	}

	// Default mode: channels need whitelisting, everything else is logged
	// unless blacklisted.
	if chatType == "channel" {
		return c.isWhitelisted(ctx, chatID, chatType)
	}
	return c.isNotBlacklisted(ctx, chatID, chatType)
}

func (c *Checker) isWhitelisted(ctx context.Context, chatID int64, chatType string) (bool, error) {
	count, err := c.queries.IsWhitelisted(ctx, sqlc.IsWhitelistedParams{
		ChatID:   chatID,
		ChatType: chatType,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (c *Checker) isNotBlacklisted(ctx context.Context, chatID int64, chatType string) (bool, error) {
	count, err := c.queries.IsBlacklisted(ctx, sqlc.IsBlacklistedParams{
		ChatID:   chatID,
		ChatType: chatType,
	})
	if err != nil {
		return false, err
	}
	return count == 0, nil
}
