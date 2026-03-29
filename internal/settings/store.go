package settings

import (
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
	return os.WriteFile(s.path, content, 0o644)
}
