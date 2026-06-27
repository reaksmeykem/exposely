# Exposely

Exposely is a Windows desktop app built with Wails that helps you expose local projects through Cloudflare Quick Tunnels without hand-editing config files or repeatedly typing `cloudflared` commands.

It supports:

- Laravel and other local-host based apps
- static HTML folders
- already running local URLs such as `http://127.0.0.1:5500`
- auto-detected local projects that can use a folder, local URL, or start command
- EnvKit ([envkit.net](https://envkit.net)) installs — detected automatically so `*.test` HTTPS sites tunnel with the right origin URL

## What Projects Work

Exposely works best with projects that are already reachable locally in one of these ways:

- a local host such as `app.test` or `my-app.local`
- a local URL such as `http://127.0.0.1:8000`, `http://127.0.0.1:5500`, or `http://localhost:5173`
- a static folder with `index.html`
- a built frontend output folder such as `dist`, `build`, or `public`

Common project types that work:

- Laravel behind Herd, Valet, Nginx, Apache, Caddy, or another local host based setup
- Laravel running with `php artisan serve`
- Laravel behind EnvKit's nginx/apache + PHP-FPM (auto-detected)
- plain HTML and static websites
- Tailwind sites that are already built or already running locally
- Vite apps
- React apps
- Vue apps
- Svelte apps
- Next.js / Node dev servers behind EnvKit (auto-detected)
- other local dev servers that expose a reachable HTTP URL

Projects are less likely to work if they are only raw source files with no running dev server, no build output, and no local URL yet.

## Features

- Detects `cloudflared` from `PATH`, a configured path, or common Windows install locations
- Stores app settings and project presets in `%AppData%\Exposely\settings.json`
- Creates quick tunnels for local URLs and static sites
- Can serve a static folder automatically when needed
- Can start a local dev command and detect the resulting local URL
- Shows live `cloudflared` and project logs in the desktop UI

## Project Types

### Auto

Auto mode can:

- use an existing local URL
- start a project command and detect a local dev server
- use a local host for local-server projects
- serve a static folder or common build output such as `dist`, `build`, or `public`

Use Auto when:

- you want the app to figure out whether the project should use a local URL, local host, start command, or static output folder
- you have a frontend project such as Vite, React, Vue, Svelte, or Tailwind and want one flexible mode
- you are using `php artisan serve` or another local server running on a port

### Laravel / Local Host

Use this when your app runs behind a local host such as:

- `my-app.test`
- `my-app.local`
- a local server fronted by Herd, Valet, or another host-based setup

This mode is not limited to Herd. It works with any local setup that responds correctly when the Host header is set, including Herd, Valet, Nginx, Apache, Caddy, and similar tools.

### HTML / Static Site

Use this when your project is:

- a static folder with `index.html`
- a built static site
- already running on a local URL such as `http://127.0.0.1:5500`

This also works well for built Tailwind sites and other frontend output folders that contain a real `index.html`.

### EnvKit (auto-detected)

If [EnvKit](https://envkit.net) is installed on the machine, Exposely detects it on launch (Windows: registry `Uninstall` entries; macOS: `/Applications/EnvKit.app`) and surfaces the install in the **Local dev stack** card under **Settings → Setup**.

When EnvKit is detected and you have not changed the default service URL, Exposely switches the cloudflared upstream to `https://127.0.0.1:443` so your trusted-HTTPS `.test` site is tunneled instead of the bare HTTP loopback. The behaviour is opt-out: as soon as you set a custom `DefaultServiceURL` (or a per-project `OriginURL`), Exposely leaves it alone.

What gets detected:

- Windows: `DisplayName` starting with `EnvKit` in the standard Uninstall registry keys (HKLM, WOW6432Node, HKCU), with `InstallLocation` and `DisplayVersion` surfaced in the UI. Falls back to known install paths if the registry entry is missing.
- macOS: `/Applications/EnvKit.app` (or `~/Applications/EnvKit.app`) plus the `~/Library/Application Support/EnvKit` data directory.
- Version comes from `DisplayVersion` on Windows and `CFBundleShortVersionString` on macOS.

You can verify detection from the CLI with:

```powershell
exposely status
```

When EnvKit is present you will see an `EnvKit:` line that lists the version, install path, and the swapped origin URL.

#### Skip upstream TLS verification (opt-in)

If EnvKit's certificate SAN list does not cover the host you want to share (e.g. it lists `*.test` but your registered sites do not include the host, or the wildcard SAN is rejected by Go's strict verifier), open **Settings → Defaults** and enable **Skip upstream TLS verification**.

When the toggle is on:

- `cloudflared` is started with `--no-tls-verify` for quick tunnels (and `noTLSVerify: true` on the ingress for named tunnels), so the upstream HTTPS handshake succeeds even if the cert does not cover the requested host.
- The pre-flight reachability check also skips TLS verification, so you do not get blocked before the share starts.

The connection is still loopback-only, so the practical risk is small, but the toggle is opt-in for a reason — leave it off for projects whose upstream certificate you actually want validated.

## Requirements

1. Install Go 1.25 or newer.
2. Install Node.js 20 or newer.
3. Install the Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

4. Install `cloudflared`.

No Cloudflare account login is required for the current Exposely release. It uses Cloudflare Quick Tunnels, so you can expose a local app with `cloudflared` without running `cloudflared login`.

## Local Development

```powershell
go mod tidy
cd frontend
npm install
cd ..
wails dev
```

## Production Build

```powershell
cd frontend
npm install
cd ..
go mod tidy
wails build
```

The production executable is written to `build/bin/`.

## Windows Installer Build

To build the Windows installer, install NSIS first and make sure `makensis.exe` is available on `PATH`.

Quick check:

```powershell
makensis /VERSION
```

Then build the installer with Wails:

```powershell
wails build -platform windows/amd64 -nsis
```

Expected output:

- `build/bin/Exposely.exe`
- `build/bin/Exposely-amd64-installer.exe`

The NSIS installer script used by Wails is:

- `build/windows/installer/project.nsi`

## CLI

This repository also includes a CLI for developers who do not want the desktop UI.

Install it from GitHub with:

```powershell
go install github.com/reaksmeykem/exposely/cmd/exposely@latest
```

Or build it locally with:

```powershell
go build -o build/bin/exposely-cli.exe ./cmd/exposely
```

Recommended quick start:

```powershell
cd D:\code\hr-system
exposely share --host hr-system.test

cd D:\code\hr-system
exposely share --url http://127.0.0.1:8000

cd D:\code\landing-page
exposely share

cd D:\code\frontend
exposely share --start "npm run dev -- --port 5173"
```

`share` runs in the foreground and keeps the tunnel alive until you stop it with `Ctrl+C`.

`update` downloads the latest CLI release from GitHub and replaces the current executable on Windows.

Other useful commands:

```powershell
exposely status
exposely projects
exposely update
```

### Share Examples

Use these commands directly from the current project folder.

Static HTML folder with `index.html`:

```powershell
cd D:\code\landing-page
exposely share
```

Laravel with local host:

```powershell
cd D:\code\hr-system
exposely share --host hr-system.test
```

Laravel with `php artisan serve` already running:

```powershell
cd D:\code\hr-system
exposely share --url http://127.0.0.1:8000
```

Frontend app where Exposely should start the dev server:

```powershell
cd D:\code\frontend
exposely share --start "npm run dev -- --port 5173"
```

## How It Works

Exposely currently uses Cloudflare Quick Tunnels only.

For local URLs or static folders, Exposely starts `cloudflared tunnel --url ...` and returns the public URL generated by Cloudflare. This version does not require a custom domain, named tunnel, DNS route, or `cloudflared login`.

## Default Settings

The app ships with generic defaults intended to be changed in the UI before use:

- Default service URL: `http://127.0.0.1:80`
- Example local host: `app.local`
- Example local URL: `http://127.0.0.1:5500`

## Open Source Notes

- Generated files such as `frontend/dist`, `frontend/node_modules`, `frontend/wailsjs`, and `build/bin` are ignored from git.
- Wails will regenerate the frontend bindings during normal `wails dev` and `wails build` workflows.
- If you fork this project, update the product metadata, placeholders, and branding to your own values.

## CI

GitHub Actions runs:

- `go test ./...`
- frontend dependency install
- frontend build

See [.github/workflows/ci.yml](./.github/workflows/ci.yml).

## GitHub Release Assets

When a GitHub release is published, GitHub Actions builds the Windows release assets and uploads them to that release:

- `Exposely.exe`
- `Exposely-amd64-installer.exe`
- `exposely-cli.exe`

The release workflow is defined in [.github/workflows/release.yml](./.github/workflows/release.yml).

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md), [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md), and [SECURITY.md](./SECURITY.md).

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).

## Mock UI

See [docs/mock-ui.svg](./docs/mock-ui.svg) for the lightweight UI mock included in the repo.
