//go:build ignore

package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		panic(err)
	}

	privateKeyBase64 := strings.TrimSpace(os.Getenv("LICENSE_PRIVATE_KEY_BASE64"))
	if privateKeyBase64 == "" {
		panic("set LICENSE_PRIVATE_KEY_BASE64 before running this command")
	}

	payloadBytes, err := os.ReadFile("payload.json")
	if err != nil {
		panic(err)
	}

	privateKey, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		panic(err)
	}

	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), payloadBytes)
	token := base64.RawURLEncoding.EncodeToString(payloadBytes) + "." + base64.RawURLEncoding.EncodeToString(signature)

	fmt.Println("LICENSE_KEY=")
	fmt.Println(token)
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
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}

	return nil
}
