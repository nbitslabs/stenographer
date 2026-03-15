package telegram

import (
	"context"

	"github.com/gotd/td/telegram/updates"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
)

type SQLiteAccessHasher struct {
	q *sqlc.Queries
}

func NewSQLiteAccessHasher(q *sqlc.Queries) *SQLiteAccessHasher {
	return &SQLiteAccessHasher{q: q}
}

var _ updates.ChannelAccessHasher = (*SQLiteAccessHasher)(nil)

func (h *SQLiteAccessHasher) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	return h.q.UpsertChannelAccessHash(ctx, sqlc.UpsertChannelAccessHashParams{
		UserID:     userID,
		ChannelID:  channelID,
		AccessHash: accessHash,
	})
}

func (h *SQLiteAccessHasher) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (int64, bool, error) {
	hash, err := h.q.GetChannelAccessHash(ctx, sqlc.GetChannelAccessHashParams{UserID: userID, ChannelID: channelID})
	if err != nil {
		return 0, false, nil // not found
	}
	return hash, true, nil
}
