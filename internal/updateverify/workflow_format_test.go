package updateverify

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/reaksmeykem/exposely/internal/sysproc"
)

// TestWorkflowChecksumFormatIsParseable guards the contract between the
// "Publish checksums" step in .github/workflows/release.yml and the
// parsers here: if the two ever drift apart, every auto-update starts
// failing with "SHA256SUMS does not list ...", which is hard to diagnose
// from the release side.
//
// The test runs the same PowerShell one-liner the workflow runs, so it
// covers the encoding quirks that a hand-written fixture would miss
// (CRLF line endings and a possible byte-order mark).
func TestWorkflowChecksumFormatIsParseable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the release workflow publishes checksums with PowerShell on Windows")
	}

	dir := t.TempDir()
	assetPath := filepath.Join(dir, "Exposely.exe")
	const assetName = "Exposely.exe"
	if err := os.WriteFile(assetPath, []byte("pretend executable payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	sumsPath := filepath.Join(dir, SumsAssetName)

	// Mirrors release.yml: lowercase hash, two spaces, asset name, ASCII file.
	script := "$hash = (Get-FileHash -LiteralPath " + sysproc.PowerShellLiteral(assetPath) +
		" -Algorithm SHA256).Hash.ToLowerInvariant(); \"$hash  " + assetName + "\" | Out-File -FilePath " +
		sysproc.PowerShellLiteral(sumsPath) + " -Encoding ascii"

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	cmd.SysProcAttr = sysproc.Hidden()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running the workflow checksum step failed: %v\n%s", err, out)
	}

	content, err := os.ReadFile(sumsPath)
	if err != nil {
		t.Fatalf("the workflow step did not write %s: %v", SumsAssetName, err)
	}

	digest, err := FindDigest(string(content), assetName)
	if err != nil {
		t.Fatalf("FindDigest could not read the workflow's own output (%q): %v", string(content), err)
	}
	if err := VerifyFile(assetPath, assetName, digest); err != nil {
		t.Fatalf("VerifyFile rejected the asset the workflow hashed: %v", err)
	}

	// And the tamper case: a changed file must not verify against that digest.
	if err := os.WriteFile(assetPath, []byte("tampered payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(assetPath, assetName, digest); err == nil {
		t.Fatal("VerifyFile accepted a file that no longer matches the published digest")
	}
}
