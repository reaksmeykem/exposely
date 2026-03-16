# Cloudflare Tunnel Manager for Laravel Herd

Windows desktop MVP for managing Cloudflare Tunnel against local Laravel Herd sites without hand-editing `config.yml` or repeatedly typing `cloudflared` commands.

## What this MVP does

- Detects `cloudflared` from `PATH`, a configured executable path, or common Windows install paths.
- Persists project presets and app defaults in `%AppData%\CloudflareTunnelManager\settings.json`.
- Reads and writes `%USERPROFILE%\.cloudflared\config.yml`.
- Supports multiple ingress rules in one named tunnel config.
- Reuses or creates a named tunnel, adds DNS routes, updates ingress safely, and starts `cloudflared tunnel run <name>`.
- Supports Laravel Herd host header override through `originRequest.httpHostHeader`.
- Shows live `cloudflared` and `npm run build` output in the UI.
- Supports:
  - stable hostnames such as `app.reaksmeykem.dev`
  - generated random hostnames under your domain such as `x8f3k2.reaksmeykem.dev`
  - Cloudflare quick tunnels via `trycloudflare.com`
- Opens public URLs and the Cloudflare config file from the UI.
- Tests the local Herd site by calling the configured origin URL with the selected `Host` header.

## Stack

- Go
- Wails v2
- TypeScript + Vite frontend
- `cloudflared` CLI subprocess execution
- YAML config management with `gopkg.in/yaml.v3`

## Project structure

```text
.
|-- app.go
|-- main.go
|-- go.mod
|-- internal
|   |-- cloudflare
|   |   |-- config.go
|   |   `-- service.go
|   |-- models
|   |   `-- types.go
|   `-- settings
|       `-- store.go
|-- frontend
|   |-- index.html
|   |-- package.json
|   |-- tsconfig.json
|   |-- vite.config.ts
|   `-- src
|       |-- api.ts
|       |-- main.ts
|       |-- style.css
|       |-- types.ts
|       `-- wails.d.ts
|-- docs
|   `-- mock-ui.svg
`-- wails.json
```

## Requirements

1. Install Go 1.25+.
2. Install Node.js 20+.
3. Install Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

4. Install `cloudflared` and log in:

```powershell
cloudflared login
```

5. If you want stable or random hostnames under your own domain, make sure:
   - the domain is on Cloudflare
   - you have permissions to create DNS records
   - the local tunnel credentials JSON exists or can be created

## Run locally

```powershell
go mod tidy
cd frontend
npm install
cd ..
wails dev
```

## Build for Windows

```powershell
cd frontend
npm install
cd ..
go mod tidy
wails build
```

## How sharing works

### Stable hostname flow

When you share a project in stable mode, the app:

1. Resolves or creates the named tunnel.
2. Runs `cloudflared tunnel route dns <tunnel-name> <hostname>`.
3. Updates `%USERPROFILE%\.cloudflared\config.yml`.
4. Adds or replaces the ingress rule for that hostname.
5. Writes:

```yaml
ingress:
  - hostname: app.reaksmeykem.dev
    service: http://127.0.0.1:80
    originRequest:
      httpHostHeader: hr-system.test
  - service: http_status:404
```

6. Starts or restarts `cloudflared tunnel --config <config> run <tunnel-name>`.

### Random hostname under your own domain

The app generates a random subdomain, adds a DNS route for it, writes the matching ingress rule, and starts the named tunnel.

### Quick tunnel mode

The app starts `cloudflared tunnel --url <service>`.

Cloudflare documents quick tunnels as development-only and notes that quick tunnels are not supported when a config file is present in the default `.cloudflared` directory. To avoid colliding with your named tunnel config, this MVP launches quick tunnel subprocesses with an isolated temporary home directory.

## Default settings

The app boots with these defaults, which you can change in the UI:

- Default domain: `reaksmeykem.dev`
- Default tunnel name: `laravel-herd`
- Default service URL: `http://127.0.0.1:80`

## MVP limitations

- No Windows system tray integration yet.
- No Windows login auto-start yet.
- No full Cloudflare API integration; the MVP relies on the CLI only.
- Quick tunnel host-header override is implemented with the CLI flag expected to map to `httpHostHeader`, but the primary Laravel Herd path should be the named tunnel modes because they use explicit ingress config.
- The frontend is written against Wails runtime globals, so opening `frontend/index.html` directly in a normal browser will not work.

## Error handling covered

- `cloudflared` missing
- tunnel missing
- DNS route failure
- invalid or missing `config.yml`
- missing project folder
- `npm run build` failure
- tunnel already running
- local service unreachable
- Cloudflare login or credential issues surfaced from `cloudflared` stderr

## Future extension points

- system tray
- Windows service install for `cloudflared`
- multiple domains
- richer tunnel inspection
- per-project service URL override
- health checks before share
- access policy management

## Mock UI

See [docs/mock-ui.svg](./docs/mock-ui.svg) for a simple UI mock derived from the shipped layout.
