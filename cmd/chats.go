package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/nbitslabs/stenographer/internal/database"
)

var chatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "List all known chats with names, message counts, and filter status",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := database.Open(cfg.Database.Path)
		if err != nil {
			return err
		}
		defer db.Close()

		rows, err := db.QueryContext(context.Background(), `
			WITH all_chats AS (
				SELECT chat_id, chat_type FROM chats
				UNION
				SELECT DISTINCT chat_id, chat_type FROM messages
			)
			SELECT
				ac.chat_id,
				ac.chat_type,
				COALESCE(c.title, '') AS title,
				COALESCE(c.username, '') AS username,
				COALESCE(msg.message_count, 0) AS message_count,
				COALESCE(msg.last_message_date, 0) AS last_message_date,
				COALESCE(
					(SELECT cf.filter_type FROM chat_filters cf
					 WHERE cf.chat_id = ac.chat_id LIMIT 1),
					''
				) AS filter_status
			FROM all_chats ac
			LEFT JOIN chats c ON c.chat_id = ac.chat_id AND c.chat_type = ac.chat_type
			LEFT JOIN (
				SELECT chat_id, chat_type, COUNT(*) AS message_count, MAX(date) AS last_message_date
				FROM messages
				GROUP BY chat_id, chat_type
			) msg ON msg.chat_id = ac.chat_id AND msg.chat_type = ac.chat_type
			ORDER BY COALESCE(msg.message_count, 0) DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "CHAT_ID\tTYPE\tNAME\tMESSAGES\tFILTER")
		fmt.Fprintln(tw, "---\t---\t---\t---\t---")

		for rows.Next() {
			var chatID, msgCount, lastDate int64
			var chatType, title, username, filterStatus string
			if err := rows.Scan(&chatID, &chatType, &title, &username, &msgCount, &lastDate, &filterStatus); err != nil {
				return err
			}

			name := title
			if username != "" {
				if name != "" {
					name += " (@" + username + ")"
				} else {
					name = "@" + username
				}
			}
			if name == "" {
				name = "-"
			}

			filter := "-"
			if filterStatus != "" {
				filter = filterStatus
			}

			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
				chatID, chatType, truncate(name, 40),
				strconv.FormatInt(msgCount, 10), filter)
		}
		return tw.Flush()
	},
}

var chatsIDsCmd = &cobra.Command{
	Use:   "ids [type]",
	Short: "Print chat IDs only (for piping to other commands)",
	Long:  "Print chat IDs, optionally filtered by type: user, chat, channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := database.Open(cfg.Database.Path)
		if err != nil {
			return err
		}
		defer db.Close()

		q := `SELECT DISTINCT chat_id, chat_type FROM chats
			  UNION SELECT DISTINCT chat_id, chat_type FROM messages`
		if len(args) > 0 {
			q = fmt.Sprintf(`SELECT DISTINCT chat_id, chat_type FROM (%s) WHERE chat_type = '%s'`, q, args[0])
		}
		q += " ORDER BY chat_id"

		rows, err := db.QueryContext(context.Background(), q)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var chatID int64
			var chatType string
			if err := rows.Scan(&chatID, &chatType); err != nil {
				return err
			}
			fmt.Println(chatID)
		}
		return nil
	},
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func init() {
	rootCmd.AddCommand(chatsCmd)
	chatsCmd.AddCommand(chatsIDsCmd)
}
