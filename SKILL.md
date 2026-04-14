---
name: stenographer
description: Query and analyze Telegram message history for knowledge base building
---

# Stenographer — Telegram Message Query Tool

Stenographer logs Telegram messages to a local SQLite database and provides CLI tools to query, filter, and export them. Use it to build structured knowledge bases from chat history.

## Quick Reference

```bash
# Discovery — run these first
stenographer resolve                    # Cache display names (run once, then periodically)
stenographer chats                      # List all chats with names and message counts
stenographer contacts                   # List people with DM history

# Querying
stenographer query recent --since 24h --resolve-names --format table
stenographer query recent --chat <ID> --since 7d --resolve-names --format jsonl
stenographer query recent --sender <ID> --since 7d --resolve-names
stenographer query recent --search "keyword" --since 7d --resolve-names
stenographer query sql "SELECT ..." --format table

# Filter management
stenographer allowlist add <chat_id>    # Start tracking a channel/supergroup
stenographer allowlist list
stenographer blacklist add <chat_id>    # Stop tracking a chat
stenographer blacklist list
```

## Important Concepts

### Chat Types
Telegram has three chat types in the database:
- **user** — Direct messages (1:1 conversations). Tracked by default.
- **chat** — Basic groups. Tracked by default.
- **channel** — Channels AND supergroups. Requires whitelisting in default mode. Most Telegram groups are supergroups internally.

### Filter Modes
- **default** (most common): Users and basic groups are logged automatically. Channels/supergroups require explicit whitelisting via `stenographer allowlist add <id>`.
- **allowlist_only**: Nothing is logged unless explicitly whitelisted.

### Smart Auto-Whitelist
When `smart_whitelist = true` (default), stenographer automatically whitelists channels/supergroups when it detects a message from someone you have DM history with. This means small work groups with known colleagues get tracked automatically.

### Name Resolution
IDs are numeric by default. To get human-readable names:
1. Run `stenographer resolve` once to cache names from Telegram API
2. Use `--resolve-names` flag on queries to include `chat_name` and `sender_name` fields
3. Names are cached in the database — subsequent queries use the cache without hitting Telegram

## Efficient Querying Patterns

### 1. Discover Before Querying
Always start by understanding what data is available:

```bash
# What chats exist and how active are they?
stenographer chats

# Who do I talk to most?
stenographer contacts

# What are the top groups by activity?
stenographer query sql "SELECT m.chat_id, c.title, COUNT(*) as msgs FROM messages m LEFT JOIN chats c ON c.chat_id = m.chat_id AND c.chat_type = m.chat_type WHERE m.chat_type IN ('chat','channel') GROUP BY m.chat_id ORDER BY msgs DESC"
```

### 2. Scoped Queries
Never pull all messages at once. Always scope by time and/or chat:

```bash
# Last 24 hours from a specific chat
stenographer query recent --chat 4885425477 --since 24h --resolve-names --format jsonl

# Last week from a specific person
stenographer query recent --sender 185795959 --since 7d --resolve-names

# Search across all chats
stenographer query recent --search "audit" --since 7d --resolve-names
```

### 3. Batch Related Chats
When building context about a project, query multiple related chats:

```bash
# Query each related chat separately for cleaner data
stenographer query recent --chat 5135987240 --since 7d --resolve-names --format jsonl > sablier_rox.jsonl
stenographer query recent --chat 5185348661 --since 7d --resolve-names --format jsonl > sablier_setup.jsonl
```

### 4. Cross-Reference People Across Chats
To understand someone's full context, query their DMs AND group messages:

```bash
# Their DMs with you
stenographer query recent --chat <user_id> --since 7d --resolve-names

# Their messages in all groups
stenographer query recent --sender <user_id> --since 7d --resolve-names
```

### 5. SQL for Complex Analysis

```bash
# Find all chats a person participates in
stenographer query sql "SELECT DISTINCT m.chat_id, c.title, m.chat_type FROM messages m LEFT JOIN chats c ON c.chat_id = m.chat_id AND c.chat_type = m.chat_type WHERE m.sender_id = 185795959 ORDER BY c.title" --format table

# Daily message volume per chat
stenographer query sql "SELECT date(date, 'unixepoch') as day, chat_id, COUNT(*) as msgs FROM messages GROUP BY day, chat_id ORDER BY day DESC, msgs DESC LIMIT 50" --format table

# Find topics discussed in a chat
stenographer query sql "SELECT message_text FROM messages WHERE chat_id = 4885425477 AND message_text != '' ORDER BY date DESC LIMIT 50"

# People overlap between two groups
stenographer query sql "SELECT DISTINCT a.sender_id, c.title as name FROM messages a JOIN messages b ON a.sender_id = b.sender_id LEFT JOIN chats c ON c.chat_id = a.sender_id AND c.chat_type = 'user' WHERE a.chat_id = 5135987240 AND b.chat_id = 5185348661"
```

### 6. Output Formats

| Format | Use Case |
|--------|----------|
| `jsonl` | Default. Best for programmatic processing — one JSON object per line |
| `json` | Full JSON array. Good for reading entire result sets |
| `table` | Human-readable. Best for quick inspection in terminal |
| `csv` | Spreadsheet import or further data processing |

Use `--fields` to select specific columns:
```bash
stenographer query recent --since 24h --format csv --fields "date,chat_id,sender_id,message_text" --resolve-names
```

## Building an Obsidian Knowledge Base

### Workflow

1. **Discovery Phase**: Use `stenographer chats` and `stenographer contacts` to understand the landscape
2. **Extract by Entity**: Query per-person and per-project, not in bulk
3. **Deduplicate Context**: A person's DMs and their group messages overlap. Extract DMs for personal context, group messages for project context
4. **Link, Don't Repeat**: Create one note per person/project, link between them

### Recommended Structure

```
knowledge/
  people/          # One file per person
  projects/        # One file per project/workstream
  groups/          # One file per Telegram group (maps to project or team)
  daily/           # Optional daily digests
```

### People Notes
For each person in `stenographer contacts`:

```bash
# Get their DM history
stenographer query recent --chat <user_id> --since 30d --resolve-names --format jsonl

# Get their group participation
stenographer query sql "SELECT DISTINCT m.chat_id, c.title FROM messages m LEFT JOIN chats c ON c.chat_id = m.chat_id AND c.chat_type = m.chat_type WHERE m.sender_id = <user_id> AND m.chat_type != 'user'" --format table
```

Create a note with:
- Name, username, user ID
- Groups they're in (as [[links]])
- Key topics from DM history
- Recent activity summary

### Project/Group Notes
For each active group in `stenographer chats`:

```bash
# Get recent discussion
stenographer query recent --chat <chat_id> --since 7d --resolve-names --format jsonl

# Get participants
stenographer query sql "SELECT DISTINCT m.sender_id, c.title as name, c.username FROM messages m LEFT JOIN chats c ON c.chat_id = m.sender_id AND c.chat_type = 'user' WHERE m.chat_id = <chat_id>" --format table
```

Create a note with:
- Group name, chat ID, type
- Participants (as [[links]] to people notes)
- Key discussion topics
- Action items and decisions

### Handling DM ↔ Group Overlap
People discuss the same topics in DMs and groups. To avoid duplication:
- **DM notes** capture personal context: opinions, requests to you, private decisions
- **Group notes** capture shared context: announcements, group decisions, status updates
- Cross-link with `[[Person Name]]` and `[[Group Name]]` Obsidian links

### Example: Building Context for "Sablier Integration"

```bash
# 1. Find related chats
stenographer query sql "SELECT m.chat_id, c.title, COUNT(*) as msgs FROM messages m LEFT JOIN chats c ON c.chat_id = m.chat_id AND c.chat_type = m.chat_type WHERE m.message_text LIKE '%sablier%' GROUP BY m.chat_id ORDER BY msgs DESC" --format table

# 2. Get discussion from the main group
stenographer query recent --chat 5185348661 --since 30d --resolve-names --format jsonl

# 3. Get related DMs from key participants
stenographer query recent --chat 185795959 --search "sablier" --since 30d --resolve-names

# 4. Get the setup/ops chat
stenographer query recent --chat 5135987240 --since 30d --resolve-names --format jsonl
```

## Tips

- Always run `stenographer resolve` before querying with `--resolve-names` — it caches names and makes subsequent queries instant
- Use `--since` with relative durations (`24h`, `7d`, `30d`) rather than absolute dates
- The `--search` flag is case-insensitive substring match by default. Use `--search-fuzzy` for LIKE pattern matching
- Use `stenographer query sql` for complex joins and aggregations that `query recent` can't express
- Message `date` field is Unix timestamp (seconds). Use `date(date, 'unixepoch')` in SQL for human dates
- The `is_outgoing` field (0/1) distinguishes messages you sent vs received
- Use `--exclude-chat` to filter out noisy bots or announcement channels from broad queries
- Chat IDs are stable — save them for repeated queries
