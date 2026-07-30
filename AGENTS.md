# Panaremote — Panasonic TV Remote PWA

## Architecture

Single Go binary that embeds the PWA frontend (`static/*`) and exposes two API endpoints:

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/discover` | GET | Returns current TV IP and connection status |
| `/api/remote` | POST | Proxies a `keyEvent` as a SOAP `X_SendKey` envelope to the TV |

The root path (`/`) serves the embedded PWA (HTML, manifest, service worker).

## How It Works

1. **Discovery** — A background goroutine uses SSDP (UPnP) to find Panasonic `MediaRenderer` devices on the LAN.
2. **Health check** — Periodically `GET`s `http://<TV-IP>:55000/nrc/control_0` to verify the TV is responsive.
3. **Command proxy** — Incoming JSON `{ "keyEvent": "NRC_VOLUP-ONOFF" }` is wrapped in a SOAP envelope and POSTed to the TV's network control endpoint.
4. **Auto-reconnect** — If health checks fail, the discovery loop restarts until the TV is found again.

## Key Files

- **`main.go`** — Entire Go server: HTTP mux, SSDP discovery, SOAP proxy, embedded static files.
- **`static/index.html`** — PWA UI (dark theme, grid of buttons, status polling).
- **`static/manifest.json`** — PWA manifest (installable as standalone app).
- **`static/sw.js`** — Minimal service worker (pass-through with cache fallback).
- **`commands.md`** — Reference of all valid `NRC_*-ONOFF` key codes for Panasonic Viera TVs.

## Dependencies

- `github.com/koron/go-ssdp` — SSDP/UPnP discovery
- `github.com/rs/cors` — CORS middleware

## Build & Run

```bash
go run .
# Server starts on :3000
```

Single-binary build:

```bash
go build -o panaremote .
```

## Conventions

- All TV key codes are `NRC_*-ONOFF` strings (see `commands.md`).
- The TV is always addressed on port `55000` at `/nrc/control_0`.
- State (`targetTVIP`, `isTVConnected`) is guarded by `ipMutex` (RWMutex).
- The PWA frontend is self-contained — no build step, no external JS/CSS frameworks.
- Keep the frontend minimal; add new button groups to the `remote-grid` in `index.html`.
