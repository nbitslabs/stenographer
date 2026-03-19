package query

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
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

// NameResolverFunc resolves a list of IDs to a name map.
type NameResolverFunc func(ids []int64) (map[int64]string, error)

// Formatter writes query results in a specified format.
type Formatter struct {
	format       string
	fields       []string
	w            io.Writer
	nameCache    map[int64]string
	nameResolver NameResolverFunc
}

// NewFormatter creates a new output formatter.
func NewFormatter(format string, fields []string, w io.Writer) *Formatter {
	return &Formatter{format: format, fields: fields, w: w}
}

// SetNameCache sets resolved names for chat/sender IDs directly.
func (f *Formatter) SetNameCache(names map[int64]string) {
	f.nameCache = names
}

// SetNameResolver sets a function to resolve IDs to names after rows are read.
func (f *Formatter) SetNameResolver(fn NameResolverFunc) {
	f.nameResolver = fn
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

	// Resolve names if a resolver is set and we don't have a cache yet.
	if f.nameResolver != nil && f.nameCache == nil {
		ids := CollectIDs(records)
		if names, err := f.nameResolver(ids); err == nil && len(names) > 0 {
			f.nameCache = names
		}
	}

	// Inject resolved names into records.
	if f.nameCache != nil {
		for _, r := range records {
			f.injectNames(r)
		}
	}

	fields := f.fields
	if len(fields) == 0 {
		fields = cols
		if f.nameCache != nil {
			fields = append(fields, "chat_name", "sender_name")
		}
	}

	stats := collectStats(records)

	switch f.format {
	case "json":
		return stats, f.writeJSON(records, fields)
	case "csv":
		return stats, f.writeCSV(records, fields)
	case "table":
		return stats, f.writeTable(records, fields)
	case "jsonl", "":
		return stats, f.writeJSONL(records, fields)
	default:
		return stats, f.writeJSONL(records, fields)
	}
}

func (f *Formatter) injectNames(record map[string]any) {
	if chatID, ok := toInt64(record["chat_id"]); ok {
		if name, found := f.nameCache[chatID]; found {
			record["chat_name"] = name
		}
	}
	if senderID, ok := toInt64(record["sender_id"]); ok {
		if name, found := f.nameCache[senderID]; found {
			record["sender_name"] = name
		}
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
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

func (f *Formatter) writeCSV(records []map[string]any, fields []string) error {
	w := csv.NewWriter(f.w)
	defer w.Flush()

	if err := w.Write(fields); err != nil {
		return err
	}

	for _, r := range records {
		row := make([]string, len(fields))
		for i, field := range fields {
			row[i] = fmt.Sprintf("%v", r[field])
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func (f *Formatter) writeTable(records []map[string]any, fields []string) error {
	// Default table columns if none specified.
	if len(f.fields) == 0 {
		fields = []string{"date", "chat_id", "sender_id", "message_text"}
		if f.nameCache != nil {
			fields = []string{"date", "chat_id", "chat_name", "sender_id", "sender_name", "message_text"}
		}
	}

	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, strings.Join(fields, "\t"))
	sep := make([]string, len(fields))
	for i := range sep {
		sep[i] = "---"
	}
	fmt.Fprintln(tw, strings.Join(sep, "\t"))

	for _, r := range records {
		vals := make([]string, len(fields))
		for i, field := range fields {
			v := fmt.Sprintf("%v", r[field])
			if field == "message_text" && len(v) > 80 {
				v = v[:77] + "..."
			}
			if field == "raw_json" && len(v) > 40 {
				v = v[:37] + "..."
			}
			vals[i] = v
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	return tw.Flush()
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
