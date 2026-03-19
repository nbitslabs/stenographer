package query

import (
	"fmt"
	"io"
	"time"
)

// Stats holds aggregate information about a query result set.
type Stats struct {
	TotalMessages int
	EarliestDate  int64
	LatestDate    int64
	UniqueChats   map[int64]bool
	UniqueSenders map[any]bool
}

// Print writes stats summary to w.
func (s *Stats) Print(w io.Writer) {
	fmt.Fprintf(w, "\n--- Query Statistics ---\n")
	fmt.Fprintf(w, "Total messages: %d\n", s.TotalMessages)
	if s.EarliestDate > 0 {
		fmt.Fprintf(w, "Time range: %s to %s\n",
			time.Unix(s.EarliestDate, 0).Format(time.RFC3339),
			time.Unix(s.LatestDate, 0).Format(time.RFC3339))
	}
	fmt.Fprintf(w, "Unique chats: %d\n", len(s.UniqueChats))
	fmt.Fprintf(w, "Unique senders: %d\n", len(s.UniqueSenders))
}

func collectStats(records []map[string]any) *Stats {
	s := &Stats{
		TotalMessages: len(records),
		UniqueChats:   make(map[int64]bool),
		UniqueSenders: make(map[any]bool),
	}

	for _, r := range records {
		if date, ok := r["date"]; ok {
			var d int64
			switch v := date.(type) {
			case int64:
				d = v
			case float64:
				d = int64(v)
			}
			if d > 0 {
				if s.EarliestDate == 0 || d < s.EarliestDate {
					s.EarliestDate = d
				}
				if d > s.LatestDate {
					s.LatestDate = d
				}
			}
		}
		if chatID, ok := r["chat_id"]; ok {
			switch v := chatID.(type) {
			case int64:
				s.UniqueChats[v] = true
			case float64:
				s.UniqueChats[int64(v)] = true
			}
		}
		if senderID, ok := r["sender_id"]; ok && senderID != nil {
			s.UniqueSenders[senderID] = true
		}
	}

	return s
}
