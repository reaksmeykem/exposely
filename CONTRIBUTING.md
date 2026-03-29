# Contributing

Thanks for contributing to Exposely.

## Development Setup

1. Install Go 1.25 or newer.
2. Install Node.js 20 or newer.
3. Install Wails:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

4. Install frontend dependencies:

```powershell
cd frontend
npm install
cd ..
```

5. Run the desktop app:

```powershell
wails dev
```

## Before Opening a Pull Request

- keep changes focused
- update docs when behavior changes
- run `go test ./...`
- run the frontend build:

```powershell
cd frontend
npm install
npm run build
cd ..
```

- verify the app still opens in `wails dev` or `wails build`

## Pull Request Guidance

- describe the user-facing change
- include screenshots or logs when UI or runtime behavior changes
- avoid bundling unrelated cleanup with feature work
- do not commit generated binaries, caches, or local secrets

## Code Style

- keep Go code straightforward and explicit
- prefer small changes over broad rewrites
- keep defaults generic and avoid personal domains, hostnames, or secrets in committed files

## Questions

Open a GitHub issue before starting large changes or refactors.
