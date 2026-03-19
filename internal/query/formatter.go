package query

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ValidFields lists valid column names for the messages table.
var ValidFields = []string{
	"id", "telegram_msg_id", "chat_id", "chat_type", "sender_id", "sender_type",
	"message_text", "date", "edit_date", "is_outgoing", "reply_to_msg_id",
	"media_type", "raw_json", "created_at", "updated_at",
}

// ValidateFields checks that all requested fields are valid column names.
func ValidateFields(fields []string) error {
	valid := make(map[string]bool, len(ValidFields))
	for _, f := range ValidFields {
		valid[f] = true
	}
	for _, f := range fields {
		if !valid[f] {
			return fmt.Errorf("invalid field %q; valid fields: %s", f, strings.Join(ValidFields, ", "))
		}
	}
	return nil
}

// Formatter writes query results in a specified format.
type Formatter struct {
	format string
	fields []string
	w      io.Writer
}

// NewFormatter creates a new output formatter.
func NewFormatter(format string, fields []string, w io.Writer) *Formatter {
	return &Formatter{format: format, fields: fields, w: w}
}

// FormatRows reads all rows from the result set and writes them in the configured format.
// Returns statistics about the result set (may be nil if no rows).
func (f *Formatter) FormatRows(rows *sql.Rows) (*Stats, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var records []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		record := make(map[string]any, len(cols))
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			record[col] = val
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, nil
	}

	fields := f.fields
	if len(fields) == 0 {
		fields = cols
	}

	stats := collectStats(records)

	switch f.format {
	case "json":
		return stats, f.writeJSON(records, fields)
	case "jsonl", "":
		return stats, f.writeJSONL(records, fields)
	default:
		return stats, f.writeJSONL(records, fields)
	}
}

func (f *Formatter) writeJSONL(records []map[string]any, fields []string) error {
	enc := json.NewEncoder(f.w)
	for _, r := range records {
		if err := enc.Encode(filterFields(r, fields)); err != nil {
			return err
		}
	}
	return nil
}

func (f *Formatter) writeJSON(records []map[string]any, fields []string) error {
	filtered := make([]map[string]any, len(records))
	for i, r := range records {
		filtered[i] = filterFields(r, fields)
	}
	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	return enc.Encode(filtered)
}

func filterFields(record map[string]any, fields []string) map[string]any {
	filtered := make(map[string]any, len(fields))
	for _, f := range fields {
		if v, ok := record[f]; ok {
			filtered[f] = v
		}
	}
	return filtered
}
