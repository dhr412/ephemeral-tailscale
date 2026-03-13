# Ephemeral Tailscale SSH Client

A portable tool for establishing temporary SSH access to remote hosts over a private Tailscale network without requiring permanent Tailscale installation or admin privileges on client machines.

## How It Works

- Creates an ephemeral tsnet node that joins your Tailscale network
- Opens a local TCP proxy
- Forwards SSH traffic through the Tailscale network to the remote host
- Uses native SSH client with all its features (keys, config, port forwarding)
- Removes all state on exit, leaving no traces of the connection.
- No system-wide Tailscale installation required

## Features

- **Zero persistent state** - All credentials and connection data removed on exit
- **No elevation required** - Runs as regular user
- **Ephemeral tailnet nodes** - Automatically removed from your network
- **Graceful cleanup** - Handles Ctrl+C (SIGINT) and SIGTERM signals
- **Cross-platform** - Tested on Windows & Linux
- **Native SSH** - Uses your existing SSH client and configuration

## Installation

**Prerequisites:**
- Go 1.22 or later (only for building from source)
- Valid Tailscale account with auth key generation access
- SSH access enabled on the remote host

**From Releases (Recommended):**

Download the latest pre-built binary for your platform from the [Releases page](https://github.com/dhr412/ephemeral-tailscale/releases).

**Linux/macOS:**
```bash
# Make it executable
chmod +x <binary path>
```

**Windows:**
```powershell
# Download the exe from releases
# Run directly or move to a directory in your PATH
```

**Build from source:**
```bash
git clone https://github.com/dhr412/ephemeral-tailscale
cd ephemeral-tailscale
go build -o tssh ./src/
```

## Usage

```bash
./tssh
```

When prompted, provide:
- Tailscale ephemeral auth key
- Host Tailscale address (e.g., `100.64.1.5` or `hostname.tailnet.ts.net`)
- Local proxy port (default: 22122)

Then connect using your SSH client:
```bash
ssh username@localhost -p 2222
```

**Help:**
```bash
./tssh --help
```

## License

This project is licensed under the MIT license.
