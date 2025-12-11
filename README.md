# mytool CLI Installer

Script installer untuk mendistribusikan CLI tool Anda.

## Struktur File

```
mytool-installer/
├── install.sh      # Installer untuk Linux/macOS
├── install.ps1     # Installer untuk Windows
└── README.md       # Dokumentasi ini
```

## Setup

### 1. Konfigurasi

Edit variabel di awal setiap script:

**install.sh:**
```sh
CLI_NAME="mytool"
CLI_VERSION="1.0.0"
GITHUB_REPO="zesbe/mytool"
```

**install.ps1:**
```powershell
$CLI_NAME = "mytool"
$CLI_VERSION = "1.0.0"
$GITHUB_REPO = "zesbe/mytool"
```

### 2. Format Binary di GitHub Releases

Upload binary dengan format nama:
```
mytool-{platform}-{arch}[.exe]
```

Contoh untuk release v1.0.0:
```
v1.0.0/
├── mytool-linux-amd64
├── mytool-linux-arm64
├── mytool-darwin-amd64
├── mytool-darwin-arm64
├── mytool-windows-amd64.exe
├── mytool-linux-amd64.sha256      (opsional)
├── mytool-linux-arm64.sha256      (opsional)
├── mytool-darwin-amd64.sha256     (opsional)
├── mytool-darwin-arm64.sha256     (opsional)
└── mytool-windows-amd64.exe.sha256 (opsional)
```

### 3. Generate Checksum (Opsional tapi Direkomendasikan)

```bash
# Linux/macOS
sha256sum mytool-linux-amd64 > mytool-linux-amd64.sha256

# atau
shasum -a 256 mytool-darwin-arm64 > mytool-darwin-arm64.sha256
```

## Hosting Script

### Opsi 1: GitHub Raw URL

Host di repo Anda dan gunakan raw URL:
```
https://raw.githubusercontent.com/zesbe/mytool/main/install.sh
```

Pengguna install dengan:
```bash
curl -fsSL https://raw.githubusercontent.com/zesbe/mytool/main/install.sh | sh
```

### Opsi 2: Custom Domain

1. Upload `install.sh` ke server/CDN Anda
2. Konfigurasi URL seperti `https://yourdomain.com/install.sh`
3. Pastikan server mengirim `Content-Type: text/plain`

Pengguna install dengan:
```bash
curl -fsSL https://yourdomain.com/install.sh | sh
```

### Opsi 3: GitHub Pages

1. Buat branch `gh-pages` atau folder `docs/`
2. Upload script installer
3. Akses via `https://zesbe.github.io/mytool/install.sh`

## Penggunaan

### Linux/macOS
```bash
curl -fsSL https://yourdomain.com/install.sh | sh
```

### Windows (PowerShell)
```powershell
irm https://yourdomain.com/install.ps1 | iex
```

## Lokasi Instalasi

| Platform | Lokasi Binary |
|----------|---------------|
| Linux    | `~/.local/bin/mytool` |
| macOS    | `~/.local/bin/mytool` |
| Windows  | `%LOCALAPPDATA%\mytool\bin\mytool.exe` |

## Build Binary dengan Go

Jika CLI Anda ditulis dengan Go:

```bash
# Build untuk semua platform
GOOS=linux GOARCH=amd64 go build -o dist/mytool-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o dist/mytool-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -o dist/mytool-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -o dist/mytool-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o dist/mytool-windows-amd64.exe .

# Generate checksum
cd dist && sha256sum * > checksums.txt
```

## Build Binary dengan Rust

```bash
# Install cross-compilation targets
rustup target add x86_64-unknown-linux-gnu
rustup target add aarch64-unknown-linux-gnu
rustup target add x86_64-apple-darwin
rustup target add aarch64-apple-darwin
rustup target add x86_64-pc-windows-msvc

# Build
cargo build --release --target x86_64-unknown-linux-gnu
cargo build --release --target x86_64-pc-windows-msvc
# ... etc
```

## GitHub Actions (Otomatis Release)

Contoh workflow untuk auto-release:

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            target: x86_64-unknown-linux-gnu
            name: mytool-linux-amd64
          - os: ubuntu-latest
            target: aarch64-unknown-linux-gnu
            name: mytool-linux-arm64
          - os: macos-latest
            target: x86_64-apple-darwin
            name: mytool-darwin-amd64
          - os: macos-latest
            target: aarch64-apple-darwin
            name: mytool-darwin-arm64
          - os: windows-latest
            target: x86_64-pc-windows-msvc
            name: mytool-windows-amd64.exe

    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - name: Build
        run: |
          # Your build commands here
          # go build / cargo build / etc

      - name: Generate checksum
        run: |
          sha256sum ${{ matrix.name }} > ${{ matrix.name }}.sha256

      - name: Upload to Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            ${{ matrix.name }}
            ${{ matrix.name }}.sha256
```

## Troubleshooting

### "Permission denied"
```bash
chmod +x ~/.local/bin/mytool
```

### "command not found" setelah install
Tambahkan ke PATH:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Windows: "execution policy"
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```
