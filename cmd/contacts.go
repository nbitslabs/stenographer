package cmd

import (
	"context"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/nbitslabs/stenographer/internal/database"
)

var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "List people you have direct message history with",
	Long:  "Shows all users with DM history, sorted by message count. Run 'stenographer resolve' first for names.",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := database.Open(cfg.Database.Path)
		if err != nil {
			return err
		}
		defer db.Close()

		rows, err := db.QueryContext(context.Background(), `
			SELECT
				m.chat_id,
				COALESCE(c.title, '') AS title,
				COALESCE(c.username, '') AS username,
				COUNT(*) AS message_count,
				SUM(CASE WHEN m.is_outgoing = 1 THEN 1 ELSE 0 END) AS sent,
				SUM(CASE WHEN m.is_outgoing = 0 THEN 1 ELSE 0 END) AS received,
				MAX(m.date) AS last_message_date
			FROM messages m
			LEFT JOIN chats c ON c.chat_id = m.chat_id AND c.chat_type = 'user'
			WHERE m.chat_type = 'user'
			GROUP BY m.chat_id
			ORDER BY message_count DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "USER_ID\tNAME\tUSERNAME\tMESSAGES\tSENT\tRECEIVED")
		fmt.Fprintln(tw, "---\t---\t---\t---\t---\t---")

		for rows.Next() {
			var chatID, msgCount, sent, received, lastDate int64
			var title, username string
			if err := rows.Scan(&chatID, &title, &username, &msgCount, &sent, &received, &lastDate); err != nil {
				return err
			}

			name := title
			if name == "" {
				name = "-"
			}

			uname := "-"
			if username != "" {
				uname = "@" + username
			}

			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
				chatID, truncate(name, 30), uname,
				strconv.FormatInt(msgCount, 10),
				strconv.FormatInt(sent, 10),
				strconv.FormatInt(received, 10))
		}
		return tw.Flush()
	},
}

func init() {
	rootCmd.AddCommand(contactsCmd)
}
