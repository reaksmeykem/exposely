# Exposely

Exposely is a Windows desktop app built with Wails that helps you expose local projects through Cloudflare Quick Tunnels without hand-editing config files or repeatedly typing `cloudflared` commands.

It supports:

- Laravel and other local-host based apps
- static HTML folders
- already running local URLs such as `http://127.0.0.1:5500`
- auto-detected local projects that can use a folder, local URL, or start command

## What Projects Work

Exposely works best with projects that are already reachable locally in one of these ways:

- a local host such as `app.test` or `my-app.local`
- a local URL such as `http://127.0.0.1:8000`, `http://127.0.0.1:5500`, or `http://localhost:5173`
- a static folder with `index.html`
- a built frontend output folder such as `dist`, `build`, or `public`

Common project types that work:

- Laravel behind Herd, Valet, Nginx, Apache, Caddy, or another local host based setup
- Laravel running with `php artisan serve`
- plain HTML and static websites
- Tailwind sites that are already built or already running locally
- Vite apps
- React apps
- Vue apps
- Svelte apps
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

Examples:

```powershell
exposely status
exposely projects
cd D:\code\hr-system
exposely init --host hr-system.test
cd D:\code\hr-system
exposely init --url http://127.0.0.1:8000
cd D:\code\frontend
exposely init --start "npm run dev -- --port 5173"
exposely project list
exposely project edit --project "hr-system" --url http://127.0.0.1:8000 --mode auto
exposely project delete --project "hr-system"
exposely update
exposely share --project my-site
cd D:\code\landing-page
exposely share
exposely share --url http://127.0.0.1:5500
cd D:\code\frontend
exposely share --start "npm run dev -- --port 5173"
```

`share` runs in the foreground and keeps the tunnel alive until you stop it with `Ctrl+C`.

`update` downloads the latest CLI release from GitHub and replaces the current executable on Windows.

### Saved Project Examples

`init` uses the current folder as the default project path and folder name, so it is the easiest way to add a saved project from the terminal.

Laravel with Herd, Valet, Nginx, Apache, Caddy, or another local host:

```powershell
cd D:\code\hr-system
exposely init --host hr-system.test
```

Laravel with `php artisan serve`:

```powershell
cd D:\code\hr-system
exposely init --url http://127.0.0.1:8000
```

Laravel where Exposely should start the local server:

```powershell
cd D:\code\hr-system
exposely init --start "php artisan serve --host=127.0.0.1 --port=8000"
```

Static HTML or Live Server:

```powershell
cd D:\code\landing-page
exposely init --url http://127.0.0.1:5500 --mode host-html
```

Vite, Tailwind, React, Vue, or another frontend dev server:

```powershell
cd D:\code\frontend
exposely init --start "npm run dev -- --port 5173"
```

If you need full control over the saved name or path, you can still use the explicit command:

```powershell
exposely project add --name "HR System" --path D:\code\hr-system --host hr-system.test --mode quick
```

### One-Shot Sharing Examples

If you do not want to save a project first, you can also share directly from the current folder:

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

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md), [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md), and [SECURITY.md](./SECURITY.md).

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).

## Mock UI

See [docs/mock-ui.svg](./docs/mock-ui.svg) for the lightweight UI mock included in the repo.
