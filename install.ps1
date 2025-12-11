# =============================================================================
# mytool CLI Installer for Windows
# Usage: irm https://yourdomain.com/install.ps1 | iex
# =============================================================================

$ErrorActionPreference = "Stop"

# ================================ CONFIG =====================================
$CLI_NAME = "mytool"
$CLI_VERSION = "1.0.0"
$GITHUB_REPO = "zesbe/mytool"
$INSTALL_DIR = "$env:LOCALAPPDATA\$CLI_NAME\bin"
# =============================================================================

function Write-Info { param($msg) Write-Host "[INFO] $msg" -ForegroundColor Cyan }
function Write-Warn { param($msg) Write-Host "[WARN] $msg" -ForegroundColor Yellow }
function Write-Err { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red; exit 1 }
function Write-Ok { param($msg) Write-Host "[OK] $msg" -ForegroundColor Green }

# Print banner
Write-Host ""
Write-Host "  +---------------------------------------+" -ForegroundColor Magenta
Write-Host "  |       $CLI_NAME Installer v$CLI_VERSION        |" -ForegroundColor Magenta
Write-Host "  +---------------------------------------+" -ForegroundColor Magenta
Write-Host ""

# Detect architecture
Write-Info "Detecting platform..."
$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    "386"
}

$arch_name = switch ($arch) {
    "amd64" { "x64" }
    "arm64" { "ARM64" }
    "386"   { "x86" }
}

Write-Ok "Detected: Windows ($arch_name)"

# Build download URL
$BINARY_NAME = "$CLI_NAME-windows-$arch.exe"
$DOWNLOAD_URL = "https://github.com/$GITHUB_REPO/releases/download/v$CLI_VERSION/$BINARY_NAME"
$CHECKSUM_URL = "$DOWNLOAD_URL.sha256"

# Create temp directory
$TMP = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }
$BINARY_PATH = Join-Path $TMP "$CLI_NAME.exe"

try {
    # Download binary
    Write-Info "Downloading $CLI_NAME v$CLI_VERSION..."
    try {
        Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $BINARY_PATH -UseBasicParsing
    } catch {
        Write-Err "Download failed. Please check:
  - Version v$CLI_VERSION exists
  - Binary '$BINARY_NAME' exists in release
  - URL: $DOWNLOAD_URL"
    }

    # Verify checksum (optional)
    Write-Info "Verifying checksum..."
    try {
        $expected = (Invoke-WebRequest -Uri $CHECKSUM_URL -UseBasicParsing).Content.Trim().Split()[0]
        $actual = (Get-FileHash -Path $BINARY_PATH -Algorithm SHA256).Hash.ToLower()

        if ($actual -ne $expected.ToLower()) {
            Write-Err "Checksum verification failed!
  Expected: $expected
  Actual:   $actual"
        }
        Write-Ok "Checksum verified"
    } catch {
        Write-Warn "Checksum file not found, skipping verification"
    }

    # Create install directory
    Write-Info "Installing to $INSTALL_DIR..."
    if (!(Test-Path $INSTALL_DIR)) {
        New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
    }

    # Stop existing process
    Get-Process -Name $CLI_NAME -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

    # Install binary
    $INSTALL_PATH = Join-Path $INSTALL_DIR "$CLI_NAME.exe"
    Copy-Item -Path $BINARY_PATH -Destination $INSTALL_PATH -Force

    Write-Ok "$CLI_NAME v$CLI_VERSION installed to $INSTALL_PATH"

    # PATH configuration
    Write-Host ""
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")

    if ($currentPath -split ";" | Where-Object { $_ -eq $INSTALL_DIR }) {
        Write-Ok "PATH already configured"
    } else {
        Write-Warn "Adding $INSTALL_DIR to PATH..."

        $newPath = "$INSTALL_DIR;$currentPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")

        # Update current session
        $env:Path = "$INSTALL_DIR;$env:Path"

        Write-Ok "PATH updated (restart terminal to apply globally)"
    }

    Write-Host ""
    Write-Host "  +---------------------------------------+" -ForegroundColor Green
    Write-Host "  |     Installation complete!           |" -ForegroundColor Green
    Write-Host "  |     Run '$CLI_NAME --help' to start    |" -ForegroundColor Green
    Write-Host "  +---------------------------------------+" -ForegroundColor Green
    Write-Host ""

} finally {
    # Cleanup temp directory
    if (Test-Path $TMP) {
        Remove-Item -Path $TMP -Recurse -Force -ErrorAction SilentlyContinue
    }
}
