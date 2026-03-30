package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/reaksmeykem/exposely/internal/cloudflare"
	"github.com/reaksmeykem/exposely/internal/models"
)

func (r *cliRunner) shareSavedProject(projectRef string) error {
	settingsValue, project, err := r.loadProject(projectRef)
	if err != nil {
		return err
	}

	fmt.Printf("Sharing project %q (%s)\n", project.DisplayName, project.ShareMode)
	return r.shareProjectWithSettings(settingsValue, project)
}

func (r *cliRunner) shareProject(project models.ProjectPreset) error {
	settingsValue, err := r.store.Load()
	if err != nil {
		return err
	}
	return r.shareProjectWithSettings(settingsValue, project)
}

func (r *cliRunner) shareProjectWithSettings(settingsValue models.AppSettings, project models.ProjectPreset) error {
	settingsValue = r.normalizeSettings(settingsValue)
	project = r.normalizeProject(project)

	if strings.TrimSpace(project.DisplayName) == "" {
		project.DisplayName = defaultManualDisplayName(project)
	}
	if err := validateCLIProject(project); err != nil {
		return err
	}

	switch normalizeCLIShareMode(project.ShareMode) {
	case models.ShareModeAuto:
		return r.startAutoTunnel(settingsValue, project)
	case models.ShareModeQuick:
		return r.startQuickTunnel(settingsValue, project)
	case models.ShareModeHostHTML:
		return r.startHTMLTunnel(settingsValue, project)
	case models.ShareModeRandomDomain:
		return r.shareProjectThroughNamedTunnel(settingsValue, project, true)
	case models.ShareModeStable:
		return r.shareProjectThroughNamedTunnel(settingsValue, project, false)
	default:
		return fmt.Errorf("unsupported share mode %q", project.ShareMode)
	}
}

func (r *cliRunner) startHTMLTunnel(settingsValue models.AppSettings, project models.ProjectPreset) error {
	path, err := r.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return err
	}

	serviceURL, port, server, err := r.resolveHTMLOrigin(project)
	if err != nil {
		return err
	}
	if server == nil {
		if err := checkCLIHTTPService(serviceURL); err != nil {
			return fmt.Errorf("HTML local URL is not reachable: %w", err)
		}
	}

	if err := r.manager.StartQuickTunnelWithHTML(path, serviceURL, "", port, server); err != nil {
		if server != nil {
			_ = server.Shutdown(context.Background())
		}
		return err
	}
	return r.waitUntilInterrupted()
}

func (r *cliRunner) startAutoTunnel(settingsValue models.AppSettings, project models.ProjectPreset) error {
	path, err := r.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return err
	}

	if strings.TrimSpace(project.StartCommand) != "" {
		serviceURL, err := r.startProjectAndDetectURL(project)
		if err != nil {
			return err
		}
		if err := r.manager.StartQuickTunnelWithHTML(path, serviceURL, "", 0, nil); err != nil {
			r.stopProjectCommand()
			return err
		}
		return r.waitUntilInterrupted()
	}

	if serviceURL, ok, err := normalizeCLIServiceURL(project.LocalURL); ok {
		if err != nil {
			return err
		}
		if err := checkCLIHTTPService(serviceURL); err != nil {
			return fmt.Errorf("local URL is not reachable: %w", err)
		}
		if err := r.manager.StartQuickTunnelWithHTML(path, serviceURL, "", 0, nil); err != nil {
			return err
		}
		return r.waitUntilInterrupted()
	}

	if strings.TrimSpace(project.LocalHost) != "" {
		return r.startQuickTunnel(settingsValue, project)
	}

	projectDir, err := r.resolveProjectDirectory(project.ProjectPath)
	if err != nil {
		return err
	}
	if detectCLILaravelProjectDir(projectDir) {
		laravelProject := project
		laravelProject.LocalHost = inferCLIHostFromProjectPath(projectDir)
		if strings.TrimSpace(laravelProject.LocalHost) == "" {
			return errors.New("Auto mode detected a Laravel project but could not infer a local host. Set --host explicitly")
		}
		return r.startQuickTunnel(settingsValue, laravelProject)
	}
	staticDir, ok := detectCLIStaticSiteDir(projectDir)
	if !ok {
		return errors.New("Auto mode could not determine how to run this project. Set a local URL, set a start command, provide a local host, or point to a folder with index.html/dist/build/public output")
	}

	serviceURL, port, server, err := r.serveStaticDirectory(staticDir)
	if err != nil {
		return err
	}
	if err := r.manager.StartQuickTunnelWithHTML(path, serviceURL, "", port, server); err != nil {
		_ = server.Shutdown(context.Background())
		return err
	}
	return r.waitUntilInterrupted()
}

func (r *cliRunner) startQuickTunnel(settingsValue models.AppSettings, project models.ProjectPreset) error {
	if strings.TrimSpace(project.LocalHost) == "" {
		return errors.New("local host is required")
	}
	path, err := r.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return err
	}
	originServiceURL, err := resolveCLIProjectOriginServiceURL(project, settingsValue.DefaultServiceURL)
	if err != nil {
		return err
	}
	if err := r.manager.StartQuickTunnel(path, originServiceURL, project.LocalHost); err != nil {
		return err
	}
	return r.waitUntilInterrupted()
}

func (r *cliRunner) shareProjectThroughNamedTunnel(settingsValue models.AppSettings, project models.ProjectPreset, useRandom bool) error {
	if strings.TrimSpace(project.LocalHost) == "" {
		return errors.New("local host is required")
	}

	path, err := r.detectCloudflaredPath(settingsValue.CloudflaredPath)
	if err != nil {
		return err
	}

	info, err := r.manager.EnsureNamedTunnel(path, settingsValue.TunnelName)
	if err != nil {
		return err
	}

	hostname, fullURL := r.resolveHostname(project, settingsValue.DefaultDomain, useRandom)
	if hostname == "" {
		return errors.New("subdomain is required for stable or random-domain sharing")
	}

	if err := r.manager.RouteDNS(path, settingsValue.TunnelName, hostname); err != nil {
		return err
	}

	cfg, cfgErr := cloudflare.ReadConfig(r.configPath)
	if cfgErr != nil && !errors.Is(cfgErr, os.ErrNotExist) {
		return cfgErr
	}
	cfg.Tunnel = info.ID
	cfg.CredentialsFile = info.CredentialsFile
	originServiceURL, err := resolveCLIProjectOriginServiceURL(project, settingsValue.DefaultServiceURL)
	if err != nil {
		return err
	}
	cloudflare.UpsertIngressRule(&cfg, cloudflare.IngressRule{
		Hostname: hostname,
		Service:  originServiceURL,
		OriginRequest: &cloudflare.OriginRequest{
			HTTPHostHeader: project.LocalHost,
		},
	})
	cloudflare.EnsureFallback(&cfg)
	if err := cloudflare.WriteConfig(r.configPath, cfg); err != nil {
		return err
	}

	if project.ID != "" {
		subdomain := strings.TrimSuffix(hostname, "."+strings.TrimSpace(settingsValue.DefaultDomain))
		settingsValue = r.updateProjectShare(settingsValue, project.ID, subdomain, fullURL)
		if err := r.store.Save(settingsValue); err != nil {
			return err
		}
	}

	if err := r.manager.StartNamedTunnel(path, r.configPath, settingsValue.TunnelName, info.ID, cloudflare.HostnamesFromConfig(cfg)); err != nil {
		return err
	}

	r.printPublicURL(fullURL)
	return r.waitUntilInterrupted()
}

func (r *cliRunner) resolveHTMLOrigin(project models.ProjectPreset) (string, int, *http.Server, error) {
	if serviceURL, ok, err := normalizeCLIServiceURL(project.LocalURL); ok {
		if err != nil {
			return "", 0, nil, err
		}
		r.printLog("html-server", "info", "Using existing local HTML server at "+serviceURL)
		return serviceURL, 0, nil, nil
	}

	projectDir, err := r.resolveProjectDirectory(project.ProjectPath)
	if err != nil {
		return "", 0, nil, err
	}
	staticDir, ok := detectCLIStaticSiteDir(projectDir)
	if !ok {
		return "", 0, nil, errors.New("HTML mode could not find index.html in the selected folder or common output folders (dist, build, public)")
	}
	return r.serveStaticDirectory(staticDir)
}

func (r *cliRunner) startProjectAndDetectURL(project models.ProjectPreset) (string, error) {
	projectDir, err := r.resolveProjectDirectory(project.ProjectPath)
	if err != nil {
		return "", err
	}

	urlCh, err := r.startProjectCommand(projectDir, project.StartCommand)
	if err != nil {
		return "", err
	}

	return r.waitForProjectServiceURL(project, urlCh)
}

func (r *cliRunner) waitForProjectServiceURL(project models.ProjectPreset, urlCh <-chan string) (string, error) {
	candidateSet := map[string]struct{}{}
	candidates := make([]string, 0, 8)
	addCandidate := func(raw string) {
		if normalized, ok, err := normalizeCLIServiceURL(raw); ok && err == nil {
			if _, exists := candidateSet[normalized]; !exists {
				candidateSet[normalized] = struct{}{}
				candidates = append(candidates, normalized)
			}
		}
	}

	addCandidate(project.LocalURL)
	if serviceURL, ok, _ := normalizeCLIServiceURL(project.ProjectPath); ok {
		addCandidate(serviceURL)
	}
	for _, port := range detectCLICommandPorts(project.StartCommand) {
		addCandidate(fmt.Sprintf("http://127.0.0.1:%d", port))
	}
	for _, port := range []int{5173, 4173, 3000, 8080, 8000, 5500, 4321, 4200, 5000} {
		addCandidate(fmt.Sprintf("http://127.0.0.1:%d", port))
	}

	timeout := time.NewTimer(25 * time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(600 * time.Millisecond)
	defer ticker.Stop()

	for {
		for _, candidate := range candidates {
			if err := checkCLIHTTPService(candidate); err == nil {
				r.printLog("project", "success", "Detected local project URL at "+candidate)
				return candidate, nil
			}
		}

		select {
		case detected := <-urlCh:
			addCandidate(detected)
		case <-ticker.C:
		case <-timeout.C:
			r.stopProjectCommand()
			return "", errors.New("could not detect a running local project URL. Set --url explicitly or use a start command that exposes a local HTTP server")
		}
	}
}

func (r *cliRunner) waitUntilInterrupted() error {
	signalCh := make(chan os.Signal, 1)
	doneCh := make(chan struct{})
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if !r.manager.Status().Running {
				close(doneCh)
				return
			}
		}
	}()

	fmt.Println(r.colorize("Tunnel is running. Press Ctrl+C to stop.", "1;32"))

	select {
	case <-signalCh:
		r.stopProjectCommand()
		return r.manager.StopTunnel()
	case <-doneCh:
		r.stopProjectCommand()
		status := r.manager.Status()
		if strings.TrimSpace(status.LastError) != "" {
			return errors.New(status.LastError)
		}
		return nil
	}
}
