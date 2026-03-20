//go:build ignore

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	applicense "cloudflaretunnelmanager/internal/license"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		panic(err)
	}

	if len(os.Args) < 2 {
		panic("usage: go run .\\verify_license.go <LICENSE_KEY>")
	}

	token := strings.TrimSpace(os.Args[1])
	publicKeyText := strings.TrimSpace(os.Getenv("CLOUDFLARE_TUNNEL_LICENSE_PUBLIC_KEY"))
	if publicKeyText == "" {
		panic("CLOUDFLARE_TUNNEL_LICENSE_PUBLIC_KEY is empty")
	}

	publicKey, err := base64.StdEncoding.DecodeString(publicKeyText)
	if err != nil {
		panic(err)
	}

	payload, err := applicense.VerifyToken(token, publicKey, "desktop-h2kipoi-lenovo", time.Now())
	if err != nil {
		panic(err)
	}

	fmt.Println("LICENSE VERIFIED")
	fmt.Println("owner:", payload.Owner)
	fmt.Println("plan:", payload.Plan)
	fmt.Println("is_admin:", payload.IsAdmin)
	fmt.Println("device_id:", payload.DeviceID)
	fmt.Println("expires_at:", payload.ExpiresAt)
}

func loadDotEnv(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		_ = os.Setenv(key, value)
	}
	return nil
}
