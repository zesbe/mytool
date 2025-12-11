#!/usr/bin/env sh
# =============================================================================
# mytool CLI Installer
# Usage: curl -fsSL https://yourdomain.com/install.sh | sh
# =============================================================================
set -e

# ================================ CONFIG =====================================
CLI_NAME="mytool"
CLI_VERSION="1.0.0"
GITHUB_REPO="zesbe/mytool"
INSTALL_DIR="$HOME/.local/bin"
# =============================================================================

# Check for curl
if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required but not installed." >&2
    exit 1
fi

# Create temp directory with cleanup trap
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Helper functions with colors
info()  { printf '\033[0;36m[INFO]\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m[WARN]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[ERROR]\033[0m %s\n' "$*"; exit 1; }
success() { printf '\033[1;32m[OK]\033[0m %s\n' "$*"; }

# Print banner
echo ""
echo "  ╔═══════════════════════════════════════╗"
echo "  ║       $CLI_NAME Installer v$CLI_VERSION        ║"
echo "  ╚═══════════════════════════════════════╝"
echo ""

# Detect platform
info "Detecting platform..."
case "$(uname -s)" in
    Darwin)
        platform="darwin"
        platform_name="macOS"
        ;;
    Linux)
        platform="linux"
        platform_name="Linux"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        error "Windows detected. Please use PowerShell installer instead:
  irm https://yourdomain.com/install.ps1 | iex"
        ;;
    *)
        error "Unsupported operating system: $(uname -s)"
        ;;
esac

# Detect architecture
case "$(uname -m)" in
    x86_64|amd64)
        arch="amd64"
        arch_name="x64"
        ;;
    arm64|aarch64)
        arch="arm64"
        arch_name="ARM64"
        ;;
    armv7l)
        arch="arm"
        arch_name="ARM"
        ;;
    *)
        error "Unsupported architecture: $(uname -m)"
        ;;
esac

success "Detected: $platform_name ($arch_name)"

# Build download URL (GitHub Releases format)
# Format: https://github.com/user/repo/releases/download/v1.0.0/mytool-linux-amd64
BINARY_NAME="${CLI_NAME}-${platform}-${arch}"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/v${CLI_VERSION}/${BINARY_NAME}"
CHECKSUM_URL="${DOWNLOAD_URL}.sha256"

info "Downloading $CLI_NAME v$CLI_VERSION..."
BINARY_PATH="$TMP/$CLI_NAME"

# Download binary
if ! curl -fsSL -o "$BINARY_PATH" "$DOWNLOAD_URL"; then
    error "Download failed. Please check:
  - Version v$CLI_VERSION exists
  - Binary '$BINARY_NAME' exists in release
  - URL: $DOWNLOAD_URL"
fi

# Verify checksum (optional - skip if not available)
info "Verifying checksum..."
if EXPECTED_SHA="$(curl -fsSL "$CHECKSUM_URL" 2>/dev/null)"; then
    # Calculate actual checksum
    if command -v sha256sum >/dev/null 2>&1; then
        ACTUAL_SHA="$(sha256sum "$BINARY_PATH" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        ACTUAL_SHA="$(shasum -a 256 "$BINARY_PATH" | awk '{print $1}')"
    else
        warn "No checksum tool found, skipping verification"
        ACTUAL_SHA=""
    fi

    if [ -n "$ACTUAL_SHA" ]; then
        # Extract just the hash if file contains "hash  filename" format
        EXPECTED_SHA="$(echo "$EXPECTED_SHA" | awk '{print $1}')"

        if [ "$ACTUAL_SHA" != "$EXPECTED_SHA" ]; then
            error "Checksum verification failed!
  Expected: $EXPECTED_SHA
  Actual:   $ACTUAL_SHA"
        fi
        success "Checksum verified"
    fi
else
    warn "Checksum file not found, skipping verification"
fi

# Make executable
chmod +x "$BINARY_PATH"

# Create install directory
info "Installing to $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR" || error "Failed to create directory: $INSTALL_DIR"

# Stop existing process (optional)
if command -v pkill >/dev/null 2>&1; then
    pkill -x "$CLI_NAME" 2>/dev/null || true
fi

# Install binary
if ! cp "$BINARY_PATH" "$INSTALL_DIR/$CLI_NAME"; then
    error "Failed to install binary. Try running with sudo?"
fi

success "$CLI_NAME v$CLI_VERSION installed to $INSTALL_DIR/$CLI_NAME"

# PATH configuration
echo ""
if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    success "PATH already configured"
else
    warn "Add $INSTALL_DIR to your PATH:"
    echo ""

    # Detect shell
    case "${SHELL##*/}" in
        zsh)
            RC_FILE="$HOME/.zshrc"
            ;;
        bash)
            if [ -f "$HOME/.bashrc" ]; then
                RC_FILE="$HOME/.bashrc"
            else
                RC_FILE="$HOME/.bash_profile"
            fi
            ;;
        fish)
            RC_FILE="$HOME/.config/fish/config.fish"
            echo "  set -gx PATH $INSTALL_DIR \$PATH"
            echo ""
            echo "  # Or run:"
            echo "  fish_add_path $INSTALL_DIR"
            ;;
        *)
            RC_FILE="$HOME/.profile"
            ;;
    esac

    if [ "${SHELL##*/}" != "fish" ]; then
        echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> $RC_FILE"
        echo "  source $RC_FILE"
    fi
fi

echo ""
echo "  ╔═══════════════════════════════════════╗"
echo "  ║     Installation complete! 🎉         ║"
echo "  ║     Run '$CLI_NAME --help' to start     ║"
echo "  ╚═══════════════════════════════════════╝"
echo ""
