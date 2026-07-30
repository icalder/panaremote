# Panaremote

Panaremote controls Panasonic Viera TVs. It runs as a Go server and serves a PWA frontend.

## Features

- The server finds Panasonic devices on the local network using SSDP.
- You can install the PWA on Android or iOS home screens.
- The application builds to a single static binary.
- The server monitors the TV connection and reconnects when the TV goes offline.

## Architecture

The Go binary does three things:
1. It embeds the HTML, CSS, and JS from `static/`.
2. A background goroutine scans the network for compatible TVs.
3. The server sends SOAP `X_SendKey` requests to the TV on port 55000.

## Build and Run

### With Go

Requires Go 1.16 or newer.

Run directly:

```bash
go run .
```

The server starts on `http://localhost:3000`. Open this URL on any device on the same network.

Build a portable binary:

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o panaremote .
```

### With Nix

Build the binary:

```bash
nix build .
```

The binary is in `result/bin/panaremote`.

Run a development shell:

```bash
nix develop
```

### NixOS Systemd Service

Add the flake input to your `flake.nix`:

```nix
inputs.panaremote.url = "github:icalder/panaremote";
```

Import the module and enable the service:

```nix
{
  imports = [
    inputs.panaremote.nixosModules.panaremote
  ];

  services.panaremote.enable = true;
}
```

This creates a `panaremote.service` unit that runs with a dynamic ephemeral user and restarts on failure. The server listens on port 3000.

## PWA Usage

1. Open the server address in a mobile browser.
2. Select **"Add to Home Screen"**.
3. Open the app from the home screen.

## Key Codes

The remote sends `NRC_*-ONOFF` key events. See [commands.md](./commands.md) for the full list.

## License

MIT
