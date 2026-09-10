package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/reaksmeykem/exposely/internal/models"
)

func TestWriteIsAtomicAndLeavesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	s := &Store{path: path}

	first := models.DefaultSettings()
	if err := s.Save(first); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Fatal("first save must not create a .bak (no prior good copy yet)")
	}

	second := models.DefaultSettings()
	second.DefaultServiceURL = "http://127.0.0.1:9999"
	if err := s.Save(second); err != nil {
		t.Fatal(err)
	}

	// .bak must hold the previous good content.
	prev, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("expected .bak after second save: %v", err)
	}
	var probe models.AppSettings
	if err := json.Unmarshal(prev, &probe); err != nil {
		t.Fatalf(".bak is not valid JSON: %v", err)
	}

	// No .tmp left behind by the rename.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind")
	}
}

func TestLoadRecoversFromCorruptMain(t *testing.T) {
	dir := t.TempDir()
	s := &Store{path: dir + "\\settings.json"}

	good := models.DefaultSettings()
	good.DefaultServiceURL = "http://127.0.0.1:9999"
	if err := s.Save(good); err != nil {
		t.Fatal(err)
	}
	// Second save creates the .bak of the first good copy.
	if err := s.Save(good); err != nil {
		t.Fatal(err)
	}

	// Simulate a torn write: zeroed bytes like the incident we hit.
	if err := os.WriteFile(s.path, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load should recover from .bak: %v", err)
	}
	if loaded.DefaultServiceURL != "http://127.0.0.1:9999" {
		t.Fatalf("recovered wrong content: %q", loaded.DefaultServiceURL)
	}
}

func TestLoadWithoutBackupStillErrors(t *testing.T) {
	dir := t.TempDir()
	s := &Store{path: dir + "\\settings.json"}
	if err := os.WriteFile(s.path, []byte("\x00\x00garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("expected error when no .bak exists")
	}
}
