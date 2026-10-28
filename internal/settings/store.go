package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/reaksmeykem/exposely/internal/models"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(appName string) (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Join(configDir, appName)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(baseDir, "settings.json")}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (models.AppSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() (models.AppSettings, error) {
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		defaults := models.DefaultSettings()
		if err := s.write(defaults); err != nil {
			return models.AppSettings{}, err
		}
		return defaults, nil
	}

	content, err := os.ReadFile(s.path)
	if err != nil {
		return models.AppSettings{}, err
	}

	var settingsValue models.AppSettings
	if err := json.Unmarshal(content, &settingsValue); err != nil {
		// The main file is corrupt (torn write, killed mid-save, two
		// instances racing). Recover from the last known-good copy
		// instead of failing the whole app.
		backup, bakErr := os.ReadFile(s.path + ".bak")
		if bakErr != nil {
			return models.AppSettings{}, err
		}
		var recovered models.AppSettings
		if json.Unmarshal(backup, &recovered) != nil {
			return models.AppSettings{}, err
		}
		settingsValue = recovered
	}
	return settingsValue, nil
}

// Update runs mutate with exclusive access to the settings file and
// persists the result atomically in one step.
//
// A plain Load→(do things)→Save pair is a lost-update trap: Wails runs
// every binding call on its own goroutine, so two overlapping actions
// both Load the same baseline and whichever saves last resurrects the
// older state — a just-saved project or tunnel URL silently disappears.
// Holding the store lock across the whole read-modify-write serialises
// them instead. The closure must not call Load/Save/Update itself (the
// lock is not re-entrant) and must not persist anything unless it
// returns nil — a non-nil error aborts the write entirely.
func (s *Store) Update(mutate func(settings *models.AppSettings) error) (models.AppSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settingsValue, err := s.loadLocked()
	if err != nil {
		return models.AppSettings{}, err
	}
	if err := mutate(&settingsValue); err != nil {
		return models.AppSettings{}, err
	}
	if err := s.write(settingsValue); err != nil {
		return models.AppSettings{}, err
	}
	return settingsValue, nil
}

func (s *Store) Save(settingsValue models.AppSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(settingsValue)
}

func (s *Store) write(settingsValue models.AppSettings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(settingsValue, "", "  ")
	if err != nil {
		return err
	}
	current, readErr := os.ReadFile(s.path)
	if readErr == nil {
		// RefreshState runs after every action and persists normalised
		// settings that are usually already on disk. Rewriting the file
		// (and its backup) on every call is pure disk churn.
		if bytes.Equal(current, content) {
			// Still guarantee a known-good backup exists: a fresh install
			// that never changes settings would otherwise have none.
			if _, statErr := os.Stat(s.path + ".bak"); errors.Is(statErr, os.ErrNotExist) {
				_ = os.WriteFile(s.path+".bak", current, 0o644)
			}
			return nil
		}
		// Snapshot the last known-good settings before overwriting. A torn
		// or zeroed settings.json (crash / force-kill / instance race) is
		// unrecoverable on its own — this file is what Load falls back to.
		var probe models.AppSettings
		if json.Unmarshal(current, &probe) == nil {
			_ = os.WriteFile(s.path+".bak", current, 0o644)
		}
	}
	// Write-then-rename keeps the live file atomic: readers see either
	// the old or the new content, never a half-written file. Same
	// volume, so the rename never falls back to a slow copy.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
