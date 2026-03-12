# Ephemeral Tailscale SSH Client

A portable tool for establishing temporary SSH access to remote hosts over a private Tailscale network without requiring permanent Tailscale installation or admin privileges on client machines. It provides two modes: an embedded tsnet-based TCP proxy that runs as a standalone binary, and a Tailscale client mode with ephemeral in-memory state.

## How It Works

**Embedded Mode (Default)**
- Creates an ephemeral tsnet node that joins your Tailscale network
- Opens a local TCP proxy
- Forwards SSH traffic through the Tailscale network to the remote host
- Uses native SSH client with all its features (keys, config, port forwarding)
- No system-wide Tailscale installation required

**Client Mode**
- Runs `tailscaled` with `--state=mem:` for ephemeral in-memory state
- Connects to Tailscale network and opens direct SSH session
- Falls back to `su` if `sudo` is unavailable
- Automatically cleans up on exit

Both modes use ephemeral authentication keys and remove all state on exit, leaving no traces of the connection.

## Features

- **Zero persistent state** - All credentials and connection data removed on exit
- **No elevation required** (embedded mode) - Runs as regular user
- **Ephemeral tailnet nodes** - Automatically removed from your network
- **Graceful cleanup** - Handles Ctrl+C (SIGINT) and SIGTERM signals
- **Cross-platform** - Embedded mode works on Windows, macOS, Linux
- **Native SSH** - Uses your existing SSH client and configuration
- **Automatic fallback** - Client mode supports both sudo and su

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
# Move to a directory in your PATH or run directly
```

**Build from source:**
```bash
git clone https://github.com/dhr412/ephemeral-tailscale
cd ephemeral-tailscale
go build -o tssh ./src/
```

## Usage

**Embedded Mode (Recommended for most users):**
```bash
# Run without arguments (embedded is default)
./tssh

# Or explicitly specify embedded mode
./tssh embedded
```

When prompted, provide:
- Tailscale ephemeral auth key
- Host Tailscale address (e.g., `100.64.1.5` or `hostname.tailnet.ts.net`)
- Local proxy port (default: 2222)

Then connect using your SSH client:
```bash
ssh username@localhost -p 2222
```

**Client Mode (For Linux VMs):**
```bash
./tssh client
```

When prompted, provide:
- Tailscale ephemeral auth key
- Host Tailscale address
- SSH username
- SSH port (default: 22)

The program will automatically establish the SSH session.

**Help:**
```bash
./tssh --help
```

## License

This project is licensed under the MIT license.
