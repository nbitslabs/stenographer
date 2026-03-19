# Stenographer

A Telegram message logger that stores messages in a local SQLite database for offline querying and analysis.

## Installation

### Quick install (Linux / macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/nbitslabs/stenographer/main/install.sh | sh
```

The script will:
1. Detect your OS and architecture
2. Download the latest binary from GitHub Releases
3. Prompt for your Telegram API credentials
4. Generate a config file at `~/.config/stenographer/config.toml`

### Non-interactive install

Set environment variables to skip prompts:

```sh
export STENOGRAPHER_APP_ID=12345
export STENOGRAPHER_APP_HASH=abcdef1234567890
export STENOGRAPHER_PHONE=+1234567890
curl -fsSL https://raw.githubusercontent.com/nbitslabs/stenographer/main/install.sh | sh
```

### Build from source

```sh
git clone https://github.com/nbitslabs/stenographer.git
cd stenographer
go build -o stenographer .
```

## Setup

### 1. Get Telegram API credentials

1. Go to [my.telegram.org](https://my.telegram.org/apps)
2. Create an application
3. Note your **App ID** and **App Hash**

### 2. Generate a config file

```sh
stenographer config init \
  --app-id YOUR_APP_ID \
  --app-hash YOUR_APP_HASH \
  --phone "+1234567890" > ~/.config/stenographer/config.toml
```

### 3. Authenticate

Run stenographer once interactively to complete the Telegram login flow:

```sh
stenographer run --config ~/.config/stenographer/config.toml
```

You'll be prompted for a login code and optional 2FA password. Once authenticated, press `Ctrl-C`.

### 4. Run as a background service

```sh
stenographer service install --config ~/.config/stenographer/config.toml
stenographer service start --config ~/.config/stenographer/config.toml
```

## Usage

### Querying messages

Retrieve recent messages:

```sh
# Last 100 messages (default)
stenographer query recent

# Last 50 messages from a specific chat
stenographer query recent --count 50 --chat -1001234567890

# Messages from the last hour
stenographer query recent --since 1h

# Search for text
stenographer query recent --search "meeting" --format table
```

Output formats: `jsonl` (default), `json`, `csv`, `table`

Filter options:
- `--chat <id>` / `--exclude-chat <id>` — filter by chat ID
- `--sender <id>` — filter by sender ID
- `--since <duration|timestamp>` — messages since a time (e.g., `15m`, `1h`, `2024-01-15`)
- `--from <timestamp>` / `--to <timestamp>` — time range
- `--search <text>` — substring search (add `--search-fuzzy` for LIKE matching)
- `--fields <field1,field2>` — select specific fields
- `--stats` — print result statistics to stderr
- `--resolve-names` — resolve IDs to names via Telegram

Run custom SQL:

```sh
stenographer query sql "SELECT chat_id, count(*) as n FROM messages GROUP BY chat_id ORDER BY n DESC LIMIT 10"
```

### Chat filtering

Control which chats are logged:

```sh
# Blacklist mode (default): log all except listed
stenographer blacklist add -1001234567890
stenographer blacklist add @username
stenographer blacklist list

# Allowlist mode: log only listed
# Set mode = "allowlist" in config.toml
stenographer allowlist add -1001234567890
```

### Service management

```sh
stenographer service status     # Check if running
stenographer service stop       # Stop the service
stenographer service restart    # Restart after config changes
stenographer service logs -f    # Follow logs
stenographer service uninstall  # Remove the service
```

## Configuration

Default config location: `~/.config/stenographer/config.toml`

```toml
[telegram]
app_id = 12345
app_hash = "your_app_hash"
phone = "+1234567890"
session_file = "~/.config/stenographer/session.json"

[database]
path = "~/.config/stenographer/stenographer.db"

[logging]
level = "info"  # debug, info, warn, error

[filter]
mode = "blacklist"  # blacklist or allowlist
```

## License

See [LICENSE](LICENSE) for details.
