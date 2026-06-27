package cloudflare

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type TunnelConfig struct {
	Tunnel          string                 `yaml:"tunnel,omitempty"`
	CredentialsFile string                 `yaml:"credentials-file,omitempty"`
	Ingress         []IngressRule          `yaml:"ingress,omitempty"`
	Extra           map[string]interface{} `yaml:",inline"`
}

type IngressRule struct {
	Hostname      string                 `yaml:"hostname,omitempty"`
	Service       string                 `yaml:"service"`
	OriginRequest *OriginRequest         `yaml:"originRequest,omitempty"`
	Extra         map[string]interface{} `yaml:",inline"`
}

type OriginRequest struct {
	HTTPHostHeader   string                 `yaml:"httpHostHeader,omitempty"`
	OriginServerName string                 `yaml:"originServerName,omitempty"`
	NoTLSVerify      bool                   `yaml:"noTLSVerify,omitempty"`
	Extra            map[string]interface{} `yaml:",inline"`
}

func ReadConfig(path string) (TunnelConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TunnelConfig{}, os.ErrNotExist
		}
		return TunnelConfig{}, err
	}

	var cfg TunnelConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return TunnelConfig{}, err
	}
	return cfg, nil
}

func WriteConfig(path string, cfg TunnelConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func UpsertIngressRule(cfg *TunnelConfig, rule IngressRule) {
	if cfg == nil {
		return
	}

	normalizedHost := strings.TrimSpace(strings.ToLower(rule.Hostname))
	newRules := make([]IngressRule, 0, len(cfg.Ingress)+1)
	fallbacks := make([]IngressRule, 0, 1)
	replaced := false

	for _, ingress := range cfg.Ingress {
		currentHost := strings.TrimSpace(strings.ToLower(ingress.Hostname))
		if ingress.Hostname == "" && strings.HasPrefix(ingress.Service, "http_status:") {
			fallbacks = append(fallbacks, ingress)
			continue
		}
		if currentHost == normalizedHost {
			newRules = append(newRules, rule)
			replaced = true
			continue
		}
		newRules = append(newRules, ingress)
	}

	if !replaced {
		newRules = append(newRules, rule)
	}
	cfg.Ingress = append(newRules, fallbacks...)
}

func EnsureFallback(cfg *TunnelConfig) {
	if cfg == nil {
		return
	}
	for _, rule := range cfg.Ingress {
		if rule.Hostname == "" && strings.TrimSpace(rule.Service) == "http_status:404" {
			return
		}
	}
	cfg.Ingress = append(cfg.Ingress, IngressRule{Service: "http_status:404"})
}

func HostnamesFromConfig(cfg TunnelConfig) []string {
	hostnames := make([]string, 0, len(cfg.Ingress))
	for _, rule := range cfg.Ingress {
		if strings.TrimSpace(rule.Hostname) != "" {
			hostnames = append(hostnames, rule.Hostname)
		}
	}
	return hostnames
}
