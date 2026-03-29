package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/models"
)

type projectFlagValues struct {
	name        string
	mode        string
	projectPath string
	localURL    string
	localHost   string
	start       string
	subdomain   string
}

func runProjectCommand(runner *cliRunner, args []string) error {
	if len(args) == 0 {
		return errors.New("project subcommand is required: list, add, edit, or delete")
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "list", "ls":
		return runner.printProjects()
	case "add", "create":
		return runProjectAddCommand(runner, args[1:])
	case "edit", "update":
		return runProjectEditCommand(runner, args[1:])
	case "delete", "remove", "rm":
		return runProjectDeleteCommand(runner, args[1:])
	default:
		return fmt.Errorf("unknown project subcommand %q", args[0])
	}
}

func runProjectAddCommand(runner *cliRunner, args []string) error {
	fs := flag.NewFlagSet("project add", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())

	values := bindProjectFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	project := models.ProjectPreset{
		DisplayName:  strings.TrimSpace(values.name),
		LocalHost:    strings.TrimSpace(values.localHost),
		ProjectPath:  strings.TrimSpace(values.projectPath),
		LocalURL:     strings.TrimSpace(values.localURL),
		StartCommand: strings.TrimSpace(values.start),
		Subdomain:    strings.TrimSpace(strings.ToLower(values.subdomain)),
	}

	mode, err := inferManualShareMode(strings.TrimSpace(values.mode), project)
	if err != nil {
		return err
	}
	project.ShareMode = mode
	if project.DisplayName == "" {
		project.DisplayName = defaultManualDisplayName(project)
	}

	saved, err := runner.createProject(project)
	if err != nil {
		return err
	}

	fmt.Printf("Saved project %q (%s)\n", saved.DisplayName, saved.ID)
	fmt.Printf("%s\n", runner.projectSummary(saved))
	return nil
}

func runProjectEditCommand(runner *cliRunner, args []string) error {
	fs := flag.NewFlagSet("project edit", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())

	projectRef := fs.String("project", "", "Saved project ID or display name")
	values := bindProjectFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ref := resolveProjectReference(*projectRef, fs.Args())
	if ref == "" {
		return errors.New("project edit requires --project <id-or-name>")
	}

	visited := visitedFlags(fs)
	if len(visited) == 0 || (len(visited) == 1 && visited["project"]) {
		return errors.New("project edit requires at least one field to change")
	}

	saved, err := runner.editProject(ref, values, visited)
	if err != nil {
		return err
	}

	fmt.Printf("Updated project %q (%s)\n", saved.DisplayName, saved.ID)
	fmt.Printf("%s\n", runner.projectSummary(saved))
	return nil
}

func runProjectDeleteCommand(runner *cliRunner, args []string) error {
	fs := flag.NewFlagSet("project delete", flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())

	projectRef := fs.String("project", "", "Saved project ID or display name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ref := resolveProjectReference(*projectRef, fs.Args())
	if ref == "" {
		return errors.New("project delete requires --project <id-or-name>")
	}

	deleted, err := runner.deleteProject(ref)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted project %q (%s)\n", deleted.DisplayName, deleted.ID)
	return nil
}

func bindProjectFlags(fs *flag.FlagSet) *projectFlagValues {
	values := &projectFlagValues{}
	fs.StringVar(&values.name, "name", "", "Project display name")
	fs.StringVar(&values.mode, "mode", "", "Share mode: auto|quick|host-html|stable|random-domain")
	fs.StringVar(&values.projectPath, "path", "", "Project folder path")
	fs.StringVar(&values.localURL, "url", "", "Local URL to expose")
	fs.StringVar(&values.localHost, "host", "", "Local host header for host-based apps")
	fs.StringVar(&values.start, "start", "", "Optional local start command for auto mode")
	fs.StringVar(&values.subdomain, "subdomain", "", "Subdomain for stable mode")
	return values
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	return visited
}

func resolveProjectReference(explicit string, positional []string) string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(strings.Join(positional, " "))
}

func applyProjectFlagValues(project *models.ProjectPreset, values *projectFlagValues, visited map[string]bool) {
	if visited["name"] {
		project.DisplayName = strings.TrimSpace(values.name)
	}
	if visited["path"] {
		project.ProjectPath = strings.TrimSpace(values.projectPath)
	}
	if visited["url"] {
		project.LocalURL = strings.TrimSpace(values.localURL)
	}
	if visited["host"] {
		project.LocalHost = strings.TrimSpace(values.localHost)
	}
	if visited["start"] {
		project.StartCommand = strings.TrimSpace(values.start)
	}
	if visited["subdomain"] {
		project.Subdomain = strings.TrimSpace(strings.ToLower(values.subdomain))
	}
}

func (r *cliRunner) createProject(project models.ProjectPreset) (models.ProjectPreset, error) {
	settingsValue, err := r.store.Load()
	if err != nil {
		return models.ProjectPreset{}, err
	}
	settingsValue = r.normalizeSettings(settingsValue)
	project = r.normalizeProject(project)

	project.ID = ensureCLIProjectID(project.ID)
	if project.DisplayName == "" {
		project.DisplayName = defaultManualDisplayName(project)
	}
	if err := validateCLIProject(project); err != nil {
		return models.ProjectPreset{}, err
	}
	if err := ensureUniqueProjectName(settingsValue, project.DisplayName, ""); err != nil {
		return models.ProjectPreset{}, err
	}

	settingsValue.Projects = append(settingsValue.Projects, project)
	if err := r.store.Save(settingsValue); err != nil {
		return models.ProjectPreset{}, err
	}
	return project, nil
}

func (r *cliRunner) editProject(projectRef string, values *projectFlagValues, visited map[string]bool) (models.ProjectPreset, error) {
	settingsValue, index, err := r.loadProjectIndex(projectRef)
	if err != nil {
		return models.ProjectPreset{}, err
	}

	project := r.normalizeProject(settingsValue.Projects[index])
	applyProjectFlagValues(&project, values, visited)

	if visited["mode"] {
		mode, err := inferManualShareMode(strings.TrimSpace(values.mode), project)
		if err != nil {
			return models.ProjectPreset{}, err
		}
		project.ShareMode = mode
	}

	if project.DisplayName == "" {
		project.DisplayName = defaultManualDisplayName(project)
	}
	project = r.normalizeProject(project)
	if err := validateCLIProject(project); err != nil {
		return models.ProjectPreset{}, err
	}
	if err := ensureUniqueProjectName(settingsValue, project.DisplayName, project.ID); err != nil {
		return models.ProjectPreset{}, err
	}

	settingsValue.Projects[index] = project
	if err := r.store.Save(settingsValue); err != nil {
		return models.ProjectPreset{}, err
	}
	return project, nil
}

func (r *cliRunner) deleteProject(projectRef string) (models.ProjectPreset, error) {
	settingsValue, index, err := r.loadProjectIndex(projectRef)
	if err != nil {
		return models.ProjectPreset{}, err
	}

	project := r.normalizeProject(settingsValue.Projects[index])
	settingsValue.Projects = append(settingsValue.Projects[:index], settingsValue.Projects[index+1:]...)
	if err := r.store.Save(settingsValue); err != nil {
		return models.ProjectPreset{}, err
	}
	return project, nil
}

func (r *cliRunner) loadProjectIndex(projectRef string) (models.AppSettings, int, error) {
	settingsValue, err := r.store.Load()
	if err != nil {
		return models.AppSettings{}, -1, err
	}
	settingsValue = r.normalizeSettings(settingsValue)

	normalizedRef := strings.TrimSpace(strings.ToLower(projectRef))
	for i, project := range settingsValue.Projects {
		if strings.EqualFold(strings.TrimSpace(project.ID), normalizedRef) || strings.EqualFold(strings.TrimSpace(project.DisplayName), normalizedRef) {
			return settingsValue, i, nil
		}
	}
	return models.AppSettings{}, -1, fmt.Errorf("project %q not found", projectRef)
}

func ensureUniqueProjectName(settingsValue models.AppSettings, displayName, ignoreID string) error {
	normalizedName := strings.TrimSpace(displayName)
	if normalizedName == "" {
		return errors.New("project display name is required")
	}
	for _, project := range settingsValue.Projects {
		if ignoreID != "" && strings.EqualFold(strings.TrimSpace(project.ID), strings.TrimSpace(ignoreID)) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(project.DisplayName), normalizedName) {
			return fmt.Errorf("a project named %q already exists", displayName)
		}
	}
	return nil
}

func ensureCLIProjectID(existing string) string {
	if strings.TrimSpace(existing) != "" {
		return strings.TrimSpace(existing)
	}
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("project-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
