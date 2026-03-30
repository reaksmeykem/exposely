package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/reaksmeykem/exposely/internal/models"
	"github.com/reaksmeykem/exposely/internal/version"
)

const cliVersion = version.Version
const cliRepoOwner = version.RepoOwner
const cliRepoName = version.RepoName

func main() {
	if len(os.Args) < 2 {
		printCLIUsage()
		os.Exit(1)
	}

	runner, err := newCLIRunner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	command := strings.ToLower(strings.TrimSpace(os.Args[1]))
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printCLIUsage()
	case "version":
		fmt.Printf("exposely %s\n", cliVersion)
	case "status":
		if err := runner.printStatus(); err != nil {
			exitWithError(err)
		}
	case "projects":
		if err := runner.printProjects(); err != nil {
			exitWithError(err)
		}
	case "init":
		if err := runInitCommand(runner, args); err != nil {
			exitWithError(err)
		}
	case "project":
		if err := runProjectCommand(runner, args); err != nil {
			exitWithError(err)
		}
	case "update", "upgrade", "self-update":
		if err := runner.selfUpdate(); err != nil {
			exitWithError(err)
		}
	case "share":
		if err := runShareCommand(runner, args); err != nil {
			exitWithError(err)
		}
	default:
		printCLIUsage()
		exitWithError(fmt.Errorf("unknown command %q", command))
	}
}

func runShareCommand(runner *cliRunner, args []string) error {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	projectRef := fs.String("project", "", "Saved project ID or display name")
	modeValue := fs.String("mode", "", "Share mode: auto|quick|host-html|stable|random-domain")
	projectPath := fs.String("path", "", "Project folder path")
	localURL := fs.String("url", "", "Local URL to expose")
	localHost := fs.String("host", "", "Local host header for host-based apps")
	originURL := fs.String("origin", "", "Origin URL for host-based apps, such as http://127.0.0.1:80")
	startCommand := fs.String("start", "", "Optional local start command for auto mode")
	subdomain := fs.String("subdomain", "", "Subdomain for stable mode")
	serviceURL := fs.String("service-url", "", "Override origin service URL for host-based apps")
	displayName := fs.String("name", "", "Display name for ad-hoc sharing")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*projectRef) != "" {
		if hasManualShareOverrides(*modeValue, *projectPath, *localURL, *localHost, *originURL, *startCommand, *subdomain, *serviceURL, *displayName) {
			return errors.New("use either --project or manual share flags, not both")
		}
		return runner.shareSavedProject(*projectRef)
	}

	project := models.ProjectPreset{
		DisplayName:  strings.TrimSpace(*displayName),
		LocalHost:    strings.TrimSpace(*localHost),
		OriginURL:    strings.TrimSpace(*originURL),
		Subdomain:    strings.TrimSpace(strings.ToLower(*subdomain)),
		ProjectPath:  strings.TrimSpace(*projectPath),
		LocalURL:     strings.TrimSpace(*localURL),
		StartCommand: strings.TrimSpace(*startCommand),
	}
	if strings.TrimSpace(*serviceURL) != "" {
		project.OriginURL = strings.TrimSpace(*serviceURL)
	}
	project, preparedMode, err := runner.applyShareDefaults(project, strings.TrimSpace(*modeValue))
	if err != nil {
		return err
	}

	mode, err := inferManualShareMode(preparedMode, project)
	if err != nil {
		return err
	}
	project.ShareMode = mode
	if project.DisplayName == "" {
		project.DisplayName = defaultManualDisplayName(project)
	}

	return runner.shareProject(project)
}

func hasManualShareOverrides(modeValue, projectPath, localURL, localHost, originURL, startCommand, subdomain, serviceURL, displayName string) bool {
	values := []string{modeValue, projectPath, localURL, localHost, originURL, startCommand, subdomain, serviceURL, displayName}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func inferManualShareMode(modeValue string, project models.ProjectPreset) (models.ShareMode, error) {
	switch strings.ToLower(strings.TrimSpace(modeValue)) {
	case "":
		switch {
		case project.Subdomain != "":
			return models.ShareModeStable, nil
		case project.LocalHost != "":
			return models.ShareModeQuick, nil
		case project.LocalURL != "" || project.ProjectPath != "" || project.StartCommand != "":
			return models.ShareModeAuto, nil
		default:
			return "", errors.New("manual sharing requires one of --project, --url, --path, --host, or --start")
		}
	case string(models.ShareModeAuto):
		return models.ShareModeAuto, nil
	case string(models.ShareModeQuick):
		return models.ShareModeQuick, nil
	case string(models.ShareModeHostHTML):
		return models.ShareModeHostHTML, nil
	case string(models.ShareModeStable):
		return models.ShareModeStable, nil
	case string(models.ShareModeRandomDomain):
		return models.ShareModeRandomDomain, nil
	default:
		return "", fmt.Errorf("unsupported share mode %q", modeValue)
	}
}

func defaultManualDisplayName(project models.ProjectPreset) string {
	switch {
	case project.LocalHost != "":
		return project.LocalHost
	case project.LocalURL != "":
		return project.LocalURL
	case project.ProjectPath != "":
		return project.ProjectPath
	case project.StartCommand != "":
		return "auto-project"
	default:
		return "manual-project"
	}
}

func printCLIUsage() {
	fmt.Println(`Exposely CLI

Usage:
  exposely status
  exposely projects
  exposely init --host hr-system.test
  exposely init --url http://127.0.0.1:8000
  exposely init --start "npm run dev -- --port 5173"
  exposely project list
  exposely project add --name "My App" --url http://127.0.0.1:5500
  exposely project add --name "HR System" --host hr-system.test --mode quick
  exposely project edit --project "HR System" --url http://127.0.0.1:8000 --mode auto
  exposely project delete --project "HR System"
  exposely share --project <id-or-name>
  exposely share
  exposely share --url http://127.0.0.1:5500
  exposely share --host app.test
  exposely share --mode host-html
  exposely share --start "npm run dev -- --port 4173"
  exposely share --host app.test --mode stable --subdomain my-app
  exposely update
  exposely version

Notes:
  - "init" uses the current folder as the project path and folder name by default.
  - "project" manages saved presets in the same settings file used by the desktop app.
  - "share" uses the current folder for obvious cases such as static HTML folders and start-command workflows.
  - "share" runs in the foreground and keeps the tunnel alive until Ctrl+C.
  - saved projects are loaded from the same settings file used by the desktop app.
  - quick, auto, and host-html modes create ephemeral public URLs.
  - stable and random-domain modes require a configured Cloudflare domain and local host.
  - "update" downloads the latest CLI release from GitHub and replaces the current executable.`)
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
