#!/bin/sh
# Stenographer installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/nbitslabs/stenographer/main/install.sh | sh
#
# Environment variables for non-interactive mode:
#   STENOGRAPHER_APP_ID    - Telegram App ID (numeric)
#   STENOGRAPHER_APP_HASH  - Telegram App Hash (alphanumeric)
#   STENOGRAPHER_PHONE     - Phone number in E.164 format (+1234567890)
#   STENOGRAPHER_CONFIG_DIR - Custom config directory (default: ~/.config/stenographer)

set -e

REPO="nbitslabs/stenographer"
BINARY_NAME="stenographer"

# --- Helpers ---

info()  { printf "  %s\n" "$@"; }
ok()    { printf "  ✓ %s\n" "$@"; }
warn()  { printf "  ⚠ %s\n" "$@" >&2; }
fatal() { printf "  ✗ %s\n" "$@" >&2; exit 1; }

# --- Platform detection ---

detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux)  OS="linux" ;;
        Darwin) OS="darwin" ;;
        *)      fatal "Unsupported operating system: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)   ARCH="amd64" ;;
        aarch64|arm64)  ARCH="arm64" ;;
        *)              fatal "Unsupported architecture: $ARCH" ;;
    esac

    info "Detected platform: ${OS}/${ARCH}"
}

# --- Version detection ---

get_latest_version() {
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | \
        grep '"tag_name"' | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        fatal "Could not determine latest release version. Check https://github.com/${REPO}/releases"
    fi

    info "Latest version: ${VERSION}"
}

# --- Binary download ---

download_binary() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${OS}-${ARCH}"
    TMPFILE=$(mktemp)

    info "Downloading ${DOWNLOAD_URL}..."
    if ! curl -fsSL -o "$TMPFILE" "$DOWNLOAD_URL"; then
        rm -f "$TMPFILE"
        fatal "Download failed. Check that a release exists for ${OS}-${ARCH} at ${DOWNLOAD_URL}"
    fi

    chmod +x "$TMPFILE"
    ok "Downloaded binary"
}

# --- Binary installation ---

install_binary() {
    INSTALL_DIR="/usr/local/bin"
    INSTALL_PATH="${INSTALL_DIR}/${BINARY_NAME}"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMPFILE" "$INSTALL_PATH"
        chmod +x "$INSTALL_PATH"
    else
        printf "  Need sudo access to install to %s. Continue? [Y/n] " "$INSTALL_DIR"
        if [ -n "$STENOGRAPHER_APP_ID" ]; then
            # Non-interactive mode: assume yes
            printf "y (non-interactive)\n"
            CONFIRM="y"
        else
            read -r CONFIRM
        fi

        case "$CONFIRM" in
            [nN]*)
                INSTALL_DIR="$HOME/.local/bin"
                INSTALL_PATH="${INSTALL_DIR}/${BINARY_NAME}"
                mkdir -p "$INSTALL_DIR"
                mv "$TMPFILE" "$INSTALL_PATH"
                chmod +x "$INSTALL_PATH"

                case ":$PATH:" in
                    *":${INSTALL_DIR}:"*) ;;
                    *) warn "${INSTALL_DIR} is not in your PATH. Add it with: export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
                esac
                ;;
            *)
                sudo mv "$TMPFILE" "$INSTALL_PATH"
                sudo chmod +x "$INSTALL_PATH"
                ;;
        esac
    fi

    ok "Installed to ${INSTALL_PATH}"
}

# --- Directory setup ---

setup_directories() {
    CONFIG_DIR="${STENOGRAPHER_CONFIG_DIR:-$HOME/.config/stenographer}"

    mkdir -p "$CONFIG_DIR"
    mkdir -p "${CONFIG_DIR}/logs"
    chmod 700 "$CONFIG_DIR"
    chmod 700 "${CONFIG_DIR}/logs"

    if [ -d "$CONFIG_DIR" ]; then
        PERMS=$(stat -c '%a' "$CONFIG_DIR" 2>/dev/null || stat -f '%A' "$CONFIG_DIR" 2>/dev/null)
        if [ "$PERMS" != "700" ]; then
            warn "Config directory permissions are ${PERMS} (recommended: 700)"
        fi
    fi

    ok "Config directory: ${CONFIG_DIR}"
}

# --- Credential collection ---

validate_app_id() {
    case "$1" in
        ''|*[!0-9]*) return 1 ;;
        *) return 0 ;;
    esac
}

validate_app_hash() {
    case "$1" in
        '') return 1 ;;
        *[!a-zA-Z0-9]*) return 1 ;;
        *) return 0 ;;
    esac
}

validate_phone() {
    case "$1" in
        +[0-9]*) return 0 ;;
        *) return 1 ;;
    esac
}

mask_hash() {
    LEN=${#1}
    if [ "$LEN" -le 4 ]; then
        printf '%s' "$1"
    else
        SHOW=$(printf '%s' "$1" | cut -c1-4)
        printf '%s%s' "$SHOW" "$(printf '%*s' $((LEN - 4)) '' | tr ' ' '*')"
    fi
}

collect_credentials() {
    # Non-interactive mode
    if [ -n "$STENOGRAPHER_APP_ID" ] && [ -n "$STENOGRAPHER_APP_HASH" ] && [ -n "$STENOGRAPHER_PHONE" ]; then
        APP_ID="$STENOGRAPHER_APP_ID"
        APP_HASH="$STENOGRAPHER_APP_HASH"
        PHONE="$STENOGRAPHER_PHONE"

        validate_app_id "$APP_ID" || fatal "Invalid STENOGRAPHER_APP_ID: must be numeric"
        validate_app_hash "$APP_HASH" || fatal "Invalid STENOGRAPHER_APP_HASH: must be alphanumeric"
        validate_phone "$PHONE" || fatal "Invalid STENOGRAPHER_PHONE: must be E.164 format (e.g., +1234567890)"

        info "Using credentials from environment variables"
        return
    fi

    # Interactive mode
    while true; do
        printf "  Enter your Telegram App ID: "
        read -r APP_ID
        validate_app_id "$APP_ID" && break
        warn "Invalid App ID: must be numeric and non-empty"
    done

    while true; do
        printf "  Enter your Telegram App Hash: "
        read -r APP_HASH
        validate_app_hash "$APP_HASH" && break
        warn "Invalid App Hash: must be alphanumeric and non-empty"
    done

    while true; do
        printf "  Enter your phone number (E.164 format, e.g., +1234567890): "
        read -r PHONE
        validate_phone "$PHONE" && break
        warn "Invalid phone: must start with + followed by digits"
    done

    # Confirmation
    printf "\n  Credentials summary:\n"
    printf "    App ID:   %s\n" "$APP_ID"
    printf "    App Hash: %s\n" "$(mask_hash "$APP_HASH")"
    printf "    Phone:    %s\n" "$PHONE"
    printf "  Confirm? [Y/n] "
    read -r CONFIRM
    case "$CONFIRM" in
        [nN]*) fatal "Aborted by user" ;;
    esac
}

# --- Config generation ---

generate_config() {
    CONFIG_FILE="${CONFIG_DIR}/config.toml"

    "$INSTALL_PATH" config init \
        --app-id "$APP_ID" \
        --app-hash "$APP_HASH" \
        --phone "$PHONE" > "$CONFIG_FILE"

    chmod 600 "$CONFIG_FILE"
    ok "Config written to ${CONFIG_FILE}"
}

# --- Verification ---

verify_install() {
    if "$INSTALL_PATH" --version >/dev/null 2>&1; then
        ok "Verified: $("$INSTALL_PATH" --version 2>&1 || echo "$INSTALL_PATH")"
    else
        ok "Binary installed (--version not yet supported)"
    fi
}

# --- Main ---

main() {
    printf "\n  Stenographer Installer\n"
    printf "  ======================\n\n"

    detect_platform
    get_latest_version
    download_binary
    install_binary
    setup_directories
    collect_credentials
    generate_config
    verify_install

    printf "\n  ✓ Stenographer installed successfully!\n"
    printf "  Installed to: %s\n" "$INSTALL_PATH"
    printf "  Config: %s\n\n" "${CONFIG_DIR}/config.toml"
    printf "  Next steps:\n"
    printf "    1. Authenticate:              stenographer run\n"
    printf "       (complete the login flow, then Ctrl-C)\n"
    printf "    2. Set up background service:  stenographer service install && stenographer service start\n"
    printf "\n  Documentation: https://github.com/${REPO}#readme\n\n"
}

main
