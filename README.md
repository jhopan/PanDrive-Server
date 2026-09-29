<div align="center">

# PanDrive Server

Multi-account Google Drive gateway in one Go binary.

[Latest Release](https://github.com/jhopan/PanDrive-Server/releases/latest) · [Download](#download) · [Quick Start](#quick-start)

</div>

## Overview

PanDrive Server combines multiple Google Drive accounts behind one web file manager. It is built for large files, split uploads, resumable transfers, quota-aware routing, and small VPS deployments.

```text
Backend     Go + SQLite WAL
Frontend    React / Vite embedded in the binary
Runtime     one binary, default 127.0.0.1:4000
Storage     Google Drive accounts, file bytes do not live on VPS disk
```

## Features

### Multi-account storage

- Connect multiple Google Drive accounts per user.
- Real-time quota tracking and configurable OAuth API safety thresholds.
- Automatic, round-robin, or account-priority upload routing.
- Server-side transfer and rebalance between Drive accounts.
- Search, gallery, storage analyzer, duplicates, trash, starred files, recent files, and activity audit.

### Large file transfers

- Resumable 32 MiB chunk uploads.
- Upload offsets persisted in SQLite and reconciled with Google after restart.
- Automatic split upload when no single account can hold a file.
- Split allocation fills the largest available accounts first.
- Bounded concurrency: 3 files, 4 chunks, and 2 split parts.
- VPS streaming merge for split downloads with HTTP Range, resume, seek, and per-part retry.
- Split health checks use Drive size and MD5 metadata. Logical checksum manifests identify the ordered part set.
- Folder upload planning preserves nested folders and reserves capacity before transfer starts.

### Sharing and access

- Multi-user isolation and admin controls.
- API keys with `pd_` prefix, hash-only storage, and instant revoke.
- Branded share pages with expiry, optional password, and optional download limit.
- Per-person Drive permissions with live access listing and revoke.
- JWT sessions, rotating refresh tokens, encrypted OAuth credentials, CSP, HSTS on HTTPS, and login rate limits.

### Operations

- Responsive PWA with light and dark themes.
- Transfer dashboard for active files, chunks, reservations, queue, and Google API pressure.
- Checksum-verified installer, rollback binary, updater, Cloudflare Tunnel, Caddy, and GHCR image support.
- Immutable versioned releases plus a rolling `latest` channel.

## Download

### Latest release

[Open latest release](https://github.com/jhopan/PanDrive-Server/releases/latest)

| Platform | Direct binary |
|---|---|
| Linux x86_64 | `https://github.com/jhopan/PanDrive-Server/releases/latest/download/pandrive-linux-amd64` |
| Linux ARM64 | `https://github.com/jhopan/PanDrive-Server/releases/latest/download/pandrive-linux-arm64` |
| Windows x86_64 | `https://github.com/jhopan/PanDrive-Server/releases/latest/download/pandrive-windows-amd64.exe` |
| Windows ARM64 | `https://github.com/jhopan/PanDrive-Server/releases/latest/download/pandrive-windows-arm64.exe` |
| macOS Intel | `https://github.com/jhopan/PanDrive-Server/releases/latest/download/pandrive-darwin-amd64` |
| macOS Apple Silicon | `https://github.com/jhopan/PanDrive-Server/releases/latest/download/pandrive-darwin-arm64` |
| Checksums | `https://github.com/jhopan/PanDrive-Server/releases/latest/download/SHA256SUMS` |

`latest` tracks the newest release. Use a pinned `vX.Y.Z` release when a fixed version is required.

### Pinned release

[PanDrive Server v1.0.0](https://github.com/jhopan/PanDrive-Server/releases/tag/v1.0.0)

Each versioned release has its own notes and immutable assets.

## Quick start

### Linux, macOS, or Termux

```bash
curl -fsSL https://raw.githubusercontent.com/jhopan/PanDrive-Server/main/deploy/install.sh | bash
```

Install a pinned release:

```bash
curl -fsSL https://raw.githubusercontent.com/jhopan/PanDrive-Server/main/deploy/install.sh | bash -s -- v1.0.0
```

The installer selects platform and architecture, verifies `SHA256SUMS`, preserves existing `.env`, and installs a system service when run as root.

### Docker

```bash
docker run -d -p 4000:4000 -v ./data:/data \
  -e JWT_ACCESS_SECRET=replace-this \
  -e TOKEN_ENCRYPTION_KEY=replace-with-32-byte-key \
  ghcr.io/jhopan/pandrive-server:latest
```

## First setup

1. Open `http://127.0.0.1:4000`.
2. Create the bootstrap account.
3. In Settings, add a Google OAuth Web Application configuration.
4. Enable Google Drive API in the Google Cloud project.
5. Register the callback URL shown by PanDrive.
6. Connect Google Drive accounts from Settings.

Local callback URL:

```text
http://localhost:4000/connected-accounts/google/callback
```

For public deployments, register the public domain callback too:

```text
https://your-domain/connected-accounts/google/callback
```

## Updating

```bash
pandrive-update --check
pandrive-update
pandrive-update --version v1.0.0
```

The updater downloads the matching binary and `SHA256SUMS`, verifies the checksum before swap, keeps the previous binary as `.prev`, restarts the service, then health-checks `/health`.

## Public deployment

PanDrive binds to `127.0.0.1:4000` by default. Use Cloudflare Tunnel for public HTTPS without inbound ports, or Caddy for direct HTTPS and automatic certificates.

The web UI and API are served by the same binary, so one release updates both together.

## Build from source

Requirements: Go and Node.js.

```bash
cd frontend
npm install
npm run build

cd ../backend-go
# Copy frontend/dist to backend-go/dist before Go build.
go test ./...
go vet ./...
go build -o pandrive .
```

## API keys

Create a key in Settings, then use it for programmatic access:

```bash
curl -H "Authorization: Bearer pd_your_key" https://your-host/api/files
```

Plaintext keys appear once. PanDrive stores only a SHA-256 hash and a short display prefix.

## Credits

PanDrive Server is independently maintained by [JhopanStore](https://github.com/jhopan).

See [CREDITS.md](CREDITS.md) for acknowledgements.
