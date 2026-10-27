package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/reaksmeykem/exposely/internal/sysproc"
	"github.com/reaksmeykem/exposely/internal/updateverify"
)

const cliReleaseAPI = "https://api.github.com/repos/" + cliRepoOwner + "/" + cliRepoName + "/releases/latest"
const cliReleasePage = "https://github.com/" + cliRepoOwner + "/" + cliRepoName + "/releases/latest"
const cliAssetName = "exposely-cli.exe"

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func (r *cliRunner) selfUpdate() error {
	if runtime.GOOS != "windows" {
		return errors.New("self-update is currently supported on Windows only")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine current executable path: %w", err)
	}

	release, err := fetchLatestCLIRelease()
	if err != nil {
		return err
	}

	latestVersion := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if latestVersion == "" {
		return errors.New("latest release does not have a usable version tag")
	}
	if latestVersion == cliVersion {
		fmt.Printf("exposely %s is already up to date.\n", cliVersion)
		return nil
	}

	assetURL := ""
	for _, asset := range release.Assets {
		if strings.EqualFold(strings.TrimSpace(asset.Name), cliAssetName) {
			assetURL = strings.TrimSpace(asset.URL)
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("latest release %s does not contain %s", release.TagName, cliAssetName)
	}

	// Resolve the published digest before downloading. The CLI replaces
	// its own executable with this file, so an unverifiable download must
	// not be installed.
	sumsURL := ""
	for _, asset := range release.Assets {
		if strings.EqualFold(strings.TrimSpace(asset.Name), updateverify.SumsAssetName) {
			sumsURL = strings.TrimSpace(asset.URL)
			break
		}
	}
	if sumsURL == "" {
		return fmt.Errorf("latest release %s does not publish %s, so %s cannot be verified — download it manually from %s",
			release.TagName, updateverify.SumsAssetName, cliAssetName, cliReleasePage)
	}

	sums, err := updateverify.FetchSums(&http.Client{Timeout: 30 * time.Second}, sumsURL, "exposely-cli/"+cliVersion)
	if err != nil {
		return err
	}
	digest, err := updateverify.FindDigest(sums, cliAssetName)
	if err != nil {
		return err
	}

	tempPath := exePath + ".download"
	if err := downloadFile(assetURL, tempPath); err != nil {
		return err
	}
	if err := updateverify.VerifyFile(tempPath, cliAssetName, digest); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	fmt.Printf("Updating exposely from %s to %s\n", cliVersion, latestVersion)
	if err := launchWindowsReplace(os.Getpid(), exePath, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	fmt.Println("Update scheduled. Open a new terminal after this command exits.")
	return nil
}

func fetchLatestCLIRelease() (releaseInfo, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, cliReleaseAPI, nil)
	if err != nil {
		return releaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "exposely-cli/"+cliVersion)

	resp, err := client.Do(req)
	if err != nil {
		return releaseInfo{}, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return releaseInfo{}, fmt.Errorf("failed to fetch latest release: GitHub responded with HTTP %d (%s)", resp.StatusCode, cliReleasePage)
	}

	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return releaseInfo{}, fmt.Errorf("failed to parse latest release response: %w", err)
	}
	return release, nil
}

func downloadFile(downloadURL, destination string) error {
	// A stalled transfer used to hang `exposely update` forever: the client
	// timeout bounds the whole request (headers plus body) and the transport
	// bounds the wait for the first byte.
	client := &http.Client{
		Timeout: 15 * time.Minute,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "exposely-cli/"+cliVersion)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download release asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download release asset: GitHub responded with HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create temporary update file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write update file: %w", err)
	}
	return nil
}

func launchWindowsReplace(parentPID int, targetPath, downloadedPath string) error {
	targetDir := filepath.Dir(targetPath)
	script := buildWindowsReplaceScript(parentPID, targetPath, downloadedPath)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", script)
	cmd.Dir = targetDir
	cmd.SysProcAttr = sysproc.Hidden()
	return cmd.Start()
}

func buildWindowsReplaceScript(parentPID int, targetPath, downloadedPath string) string {
	escapedTarget := sysproc.PowerShellLiteral(targetPath)
	escapedDownloaded := sysproc.PowerShellLiteral(downloadedPath)

	return fmt.Sprintf(`$ErrorActionPreference = 'SilentlyContinue'
$parentPID = %d
$target = %s
$downloaded = %s
for ($i = 0; $i -lt 120; $i++) {
  $parent = Get-Process -Id $parentPID -ErrorAction SilentlyContinue
  if ($null -ne $parent) {
    Start-Sleep -Milliseconds 500
    continue
  }

  try {
    Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
    Move-Item -LiteralPath $downloaded -Destination $target -Force
    if ((Test-Path -LiteralPath $target) -and -not (Test-Path -LiteralPath $downloaded)) {
      exit 0
    }
  } catch {
  }

  Start-Sleep -Milliseconds 500
}
exit 1`, parentPID, escapedTarget, escapedDownloaded)
}
