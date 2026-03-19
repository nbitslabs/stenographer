package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nbitslabs/stenographer/internal/database"
	"github.com/nbitslabs/stenographer/internal/query"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query stored messages",
}

// Shared query flags.
var (
	queryChatIDs        []int64
	queryExcludeChatIDs []int64
	querySenderIDs      []int64
	querySince          string
	queryFrom           string
	queryTo             string
	querySearch         string
	querySearchFuzzy    bool
	queryLimit          int
	queryOffset         int
	queryCount          int
	queryFormat         string
	queryFields         string
	queryStats          bool
	queryResolveNames   bool
)

var queryRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Retrieve recent messages",
	Long:  "Retrieve the most recent messages with optional filters for chat, sender, time range, and text search.",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := database.Open(cfg.Database.Path)
		if err != nil {
			return err
		}
		defer db.Close()

		f := query.Filters{
			ChatIDs:        queryChatIDs,
			ExcludeChatIDs: queryExcludeChatIDs,
			SenderIDs:      querySenderIDs,
			Since:          querySince,
			From:           queryFrom,
			To:             queryTo,
			Search:         querySearch,
			SearchFuzzy:    querySearchFuzzy,
			Limit:          queryLimit,
			Offset:         queryOffset,
			Count:          queryCount,
		}

		exec := query.NewExecutor(db)
		rows, err := exec.QueryRecent(context.Background(), f)
		if err != nil {
			return err
		}
		defer rows.Close()

		fields := parseFields(queryFields)
		if len(fields) > 0 {
			if err := query.ValidateFields(fields); err != nil {
				return err
			}
		}

		formatter := query.NewFormatter(queryFormat, fields, os.Stdout)
		stats, err := formatter.FormatRows(rows)
		if err != nil {
			return err
		}

		if queryStats && stats != nil {
			stats.Print(os.Stderr)
		}

		return nil
	},
}

var querySQLCmd = &cobra.Command{
	Use:   "sql <query>",
	Short: "Execute a raw SQL query",
	Long:  "Execute a custom SQL query against the database. SELECT results are output as JSONL by default.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := database.Open(cfg.Database.Path)
		if err != nil {
			return err
		}
		defer db.Close()

		exec := query.NewExecutor(db)
		rows, result, err := exec.QuerySQL(context.Background(), args[0])
		if err != nil {
			return err
		}

		// Non-SELECT statement: print affected rows.
		if rows == nil {
			affected, _ := result.RowsAffected()
			fmt.Printf("%d rows affected\n", affected)
			return nil
		}
		defer rows.Close()

		fields := parseFields(queryFields)
		formatter := query.NewFormatter(queryFormat, fields, os.Stdout)
		stats, err := formatter.FormatRows(rows)
		if err != nil {
			return err
		}

		if queryStats && stats != nil {
			stats.Print(os.Stderr)
		}

		return nil
	},
}

func parseFields(s string) []string {
	if s == "" {
		return nil
	}
	var fields []string
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			fields = append(fields, f)
		}
	}
	return fields
}

func addQueryFlags(cmd *cobra.Command) {
	cmd.Flags().Int64SliceVar(&queryChatIDs, "chat", nil, "Filter by chat ID (can be specified multiple times)")
	cmd.Flags().Int64SliceVar(&queryExcludeChatIDs, "exclude-chat", nil, "Exclude chat ID (can be specified multiple times)")
	cmd.Flags().Int64SliceVar(&querySenderIDs, "sender", nil, "Filter by sender ID")
	cmd.Flags().StringVar(&querySince, "since", "", "Messages since duration (e.g., 15m, 1h, 24h) or ISO 8601 timestamp")
	cmd.Flags().StringVar(&queryFrom, "from", "", "Messages from timestamp (ISO 8601 or epoch seconds)")
	cmd.Flags().StringVar(&queryTo, "to", "", "Messages to timestamp (ISO 8601 or epoch seconds)")
	cmd.Flags().StringVar(&querySearch, "search", "", "Search for text in messages")
	cmd.Flags().BoolVar(&querySearchFuzzy, "search-fuzzy", false, "Use fuzzy (LIKE) matching for search")
	cmd.Flags().IntVar(&queryLimit, "limit", 0, "Maximum number of results (default 100)")
	cmd.Flags().IntVar(&queryOffset, "offset", 0, "Skip first N results")
	cmd.Flags().StringVar(&queryFormat, "format", "jsonl", "Output format: jsonl, json, csv, table")
	cmd.Flags().StringVar(&queryFields, "fields", "", "Comma-separated list of fields to include")
	cmd.Flags().BoolVar(&queryStats, "stats", false, "Print query statistics to stderr")
	cmd.Flags().BoolVar(&queryResolveNames, "resolve-names", false, "Resolve chat/sender IDs to names via Telegram")
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.AddCommand(queryRecentCmd)
	queryCmd.AddCommand(querySQLCmd)

	queryRecentCmd.Flags().IntVar(&queryCount, "count", 0, "Number of recent messages to retrieve (default 100)")
	addQueryFlags(queryRecentCmd)

	querySQLCmd.Flags().StringVar(&queryFormat, "format", "jsonl", "Output format: jsonl, json, csv, table")
	querySQLCmd.Flags().StringVar(&queryFields, "fields", "", "Comma-separated list of fields to include")
	querySQLCmd.Flags().BoolVar(&queryStats, "stats", false, "Print query statistics to stderr")
}
