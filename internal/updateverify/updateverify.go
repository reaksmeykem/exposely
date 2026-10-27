// Package updateverify checks that a release asset downloaded by the
// updaters matches the SHA-256 digest published alongside it.
//
// Both Exposely updaters (the desktop app and `exposely update`) fetch a
// binary from GitHub Releases and then execute it — the desktop one
// replaces Exposely.exe or launches the installer, the CLI one replaces
// itself. Until now the download was trusted purely because it came over
// TLS; verifying it against the release's own SHA256SUMS asset adds a
// second, independent check that catches a truncated or swapped payload.
//
// The digest lives in the same release as the asset, so this is not a
// defence against someone who can publish releases: it defends against a
// corrupted download and against tampering with the asset in transit or
// in storage. The updaters require the file to be present and to list the
// asset, so a release that forgets to publish SHA256SUMS fails loudly
// instead of silently skipping verification.
package updateverify

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// SumsAssetName is the release asset that carries the published digests,
// in the `<hex>  <filename>` format produced by sha256sum.
const SumsAssetName = "SHA256SUMS"

// maxSumsBytes caps how much of the checksum file is accepted; a real one
// is a few hundred bytes, so anything larger is a red flag.
const maxSumsBytes = 1 << 20

// FetchSums downloads and returns the contents of the checksum asset.
func FetchSums(client *http.Client, url, userAgent string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download %s: %w", SumsAssetName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download %s: HTTP %d", SumsAssetName, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSumsBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", SumsAssetName, err)
	}
	return string(data), nil
}

// FindDigest returns the SHA-256 hex digest listed for assetName in the
// contents of a SHA256SUMS file. Matching ignores case and an optional
// leading `*` (binary mode) or `./` prefix, the way sha256sum output is
// commonly written.
func FindDigest(sums, assetName string) (string, error) {
	wanted := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(assetName)), "./")
	if wanted == "" {
		return "", fmt.Errorf("%s does not name an asset", SumsAssetName)
	}

	// A checksum file written by a Windows shell sometimes starts with a
	// byte-order mark; without stripping it the first digest would fail the
	// hex check and the whole file would look unusable.
	sums = strings.TrimPrefix(sums, "\ufeff")

	scanner := bufio.NewScanner(strings.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 2 {
			continue
		}
		digest := strings.ToLower(fields[0])
		if len(digest) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(digest); err != nil {
			continue
		}

		// The filename is everything after the digest so names with
		// spaces survive; `*name` and `./name` are stripped.
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(scanner.Text()), fields[0]))
		name = strings.TrimPrefix(name, "*")
		name = strings.TrimPrefix(strings.ToLower(name), "./")
		if name == wanted {
			return digest, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", SumsAssetName, err)
	}
	return "", fmt.Errorf("%s does not list %s", SumsAssetName, assetName)
}

// DigestFile returns the SHA-256 hex digest of a file on disk.
func DigestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyFile checks a downloaded file against the digest published for it
// and returns a descriptive error when they differ.
func VerifyFile(path, assetName, expectedDigest string) error {
	actual, err := DigestFile(path)
	if err != nil {
		return fmt.Errorf("failed to hash %s: %w", assetName, err)
	}
	if !strings.EqualFold(actual, expectedDigest) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, downloaded file is %s — the download was discarded, try again or install manually from the releases page",
			assetName, expectedDigest, actual)
	}
	return nil
}
