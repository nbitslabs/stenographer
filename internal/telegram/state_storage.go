package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/telegram/updates"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
)

type SQLiteStateStorage struct {
	q *sqlc.Queries
}

func NewSQLiteStateStorage(q *sqlc.Queries) *SQLiteStateStorage {
	return &SQLiteStateStorage{q: q}
}

var _ updates.StateStorage = (*SQLiteStateStorage)(nil)

func (s *SQLiteStateStorage) GetState(ctx context.Context, userID int64) (updates.State, bool, error) {
	row, err := s.q.GetUpdateState(ctx, userID)
	if err != nil {
		return updates.State{}, false, nil // not found
	}
	return updates.State{
		Pts:  int(row.Pts),
		Qts:  int(row.Qts),
		Date: int(row.Date),
		Seq:  int(row.Seq),
	}, true, nil
}

func (s *SQLiteStateStorage) SetState(ctx context.Context, userID int64, state updates.State) error {
	return s.q.UpsertUpdateState(ctx, sqlc.UpsertUpdateStateParams{
		UserID: userID,
		Pts:    int64(state.Pts),
		Qts:    int64(state.Qts),
		Date:   int64(state.Date),
		Seq:    int64(state.Seq),
	})
}

func (s *SQLiteStateStorage) SetPts(ctx context.Context, userID int64, pts int) error {
	res, err := s.q.UpdatePts(ctx, sqlc.UpdatePtsParams{Pts: int64(pts), UserID: userID})
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *SQLiteStateStorage) SetQts(ctx context.Context, userID int64, qts int) error {
	res, err := s.q.UpdateQts(ctx, sqlc.UpdateQtsParams{Qts: int64(qts), UserID: userID})
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *SQLiteStateStorage) SetDate(ctx context.Context, userID int64, date int) error {
	res, err := s.q.UpdateDate(ctx, sqlc.UpdateDateParams{Date: int64(date), UserID: userID})
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *SQLiteStateStorage) SetSeq(ctx context.Context, userID int64, seq int) error {
	res, err := s.q.UpdateSeq(ctx, sqlc.UpdateSeqParams{Seq: int64(seq), UserID: userID})
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *SQLiteStateStorage) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	res, err := s.q.UpdateDateSeq(ctx, sqlc.UpdateDateSeqParams{
		Date:   int64(date),
		Seq:    int64(seq),
		UserID: userID,
	})
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *SQLiteStateStorage) GetChannelPts(ctx context.Context, userID, channelID int64) (int, bool, error) {
	pts, err := s.q.GetChannelPts(ctx, sqlc.GetChannelPtsParams{UserID: userID, ChannelID: channelID})
	if err != nil {
		return 0, false, nil // not found
	}
	return int(pts), true, nil
}

func (s *SQLiteStateStorage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	return s.q.UpsertChannelPts(ctx, sqlc.UpsertChannelPtsParams{
		UserID:    userID,
		ChannelID: channelID,
		Pts:       int64(pts),
	})
}

func (s *SQLiteStateStorage) ForEachChannels(ctx context.Context, userID int64, f func(ctx context.Context, channelID int64, pts int) error) error {
	rows, err := s.q.ListChannelStates(ctx, userID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := f(ctx, row.ChannelID, int(row.Pts)); err != nil {
			return err
		}
	}
	return nil
}

func checkRowsAffected(res interface{ RowsAffected() (int64, error) }) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("state not found")
	}
	return nil
}
