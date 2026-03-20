//go:build ignore

package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func main() {
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
