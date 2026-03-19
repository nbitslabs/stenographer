package query

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Filters holds the query parameters for structured queries.
type Filters struct {
	ChatIDs        []int64
	ExcludeChatIDs []int64
	SenderIDs      []int64
	Since          string // duration (e.g. "15m", "1h") or ISO 8601 timestamp
	From           string // timestamp (ISO 8601 or epoch seconds)
	To             string // timestamp (ISO 8601 or epoch seconds)
	Search         string
	SearchFuzzy    bool
	Limit          int
	Offset         int
	Count          int // alias for limit, used by "recent" command
}

// Executor builds and executes queries against the messages table.
type Executor struct {
	db *sql.DB
}

// NewExecutor creates a new query executor.
func NewExecutor(db *sql.DB) *Executor {
	return &Executor{db: db}
}

// allColumns is the full column list for the messages table.
const allColumns = "id, telegram_msg_id, chat_id, chat_type, sender_id, sender_type, message_text, date, edit_date, is_outgoing, reply_to_msg_id, media_type, raw_json, created_at, updated_at"

// QueryRecent builds and executes a structured query with filters.
func (e *Executor) QueryRecent(ctx context.Context, f Filters) (*sql.Rows, error) {
	query, args, err := buildQuery(f)
	if err != nil {
		return nil, err
	}
	return e.db.QueryContext(ctx, query, args...)
}

// QuerySQL executes a raw SQL query. Returns rows for SELECT, or executes for other statements.
func (e *Executor) QuerySQL(ctx context.Context, rawSQL string) (*sql.Rows, sql.Result, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(rawSQL))
	if strings.HasPrefix(trimmed, "SELECT") {
		rows, err := e.db.QueryContext(ctx, rawSQL)
		return rows, nil, err
	}
	result, err := e.db.ExecContext(ctx, rawSQL)
	return nil, result, err
}

func buildQuery(f Filters) (string, []any, error) {
	var conditions []string
	var args []any

	// Chat inclusion filter.
	if len(f.ChatIDs) > 0 {
		placeholders := make([]string, len(f.ChatIDs))
		for i, id := range f.ChatIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, fmt.Sprintf("chat_id IN (%s)", strings.Join(placeholders, ",")))
	}

	// Chat exclusion filter.
	if len(f.ExcludeChatIDs) > 0 {
		placeholders := make([]string, len(f.ExcludeChatIDs))
		for i, id := range f.ExcludeChatIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, fmt.Sprintf("chat_id NOT IN (%s)", strings.Join(placeholders, ",")))
	}

	// Sender filter.
	if len(f.SenderIDs) > 0 {
		placeholders := make([]string, len(f.SenderIDs))
		for i, id := range f.SenderIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, fmt.Sprintf("sender_id IN (%s)", strings.Join(placeholders, ",")))
	}

	// --since (duration or timestamp).
	if f.Since != "" {
		ts, err := ParseSince(f.Since)
		if err != nil {
			return "", nil, fmt.Errorf("invalid --since value %q: %w", f.Since, err)
		}
		conditions = append(conditions, "date >= ?")
		args = append(args, ts)
	}

	// --from timestamp.
	if f.From != "" {
		ts, err := ParseTimestamp(f.From)
		if err != nil {
			return "", nil, fmt.Errorf("invalid --from value %q: %w", f.From, err)
		}
		conditions = append(conditions, "date >= ?")
		args = append(args, ts)
	}

	// --to timestamp.
	if f.To != "" {
		ts, err := ParseTimestamp(f.To)
		if err != nil {
			return "", nil, fmt.Errorf("invalid --to value %q: %w", f.To, err)
		}
		conditions = append(conditions, "date <= ?")
		args = append(args, ts)
	}

	// Text search.
	if f.Search != "" {
		if f.SearchFuzzy {
			conditions = append(conditions, "message_text LIKE ?")
			args = append(args, "%"+f.Search+"%")
		} else {
			conditions = append(conditions, "INSTR(LOWER(message_text), LOWER(?)) > 0")
			args = append(args, f.Search)
		}
	}

	// Build full query.
	q := "SELECT " + allColumns + " FROM messages"
	if len(conditions) > 0 {
		q += " WHERE " + strings.Join(conditions, " AND ")
	}
	q += " ORDER BY date DESC"

	// Limit.
	limit := f.Limit
	if f.Count > 0 {
		limit = f.Count
	}
	if limit <= 0 {
		limit = 100
	}
	q += fmt.Sprintf(" LIMIT %d", limit)

	// Offset.
	if f.Offset > 0 {
		q += fmt.Sprintf(" OFFSET %d", f.Offset)
	}

	return q, args, nil
}
