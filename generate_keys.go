//go:build ignore

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	fmt.Println("PUBLIC_KEY_BASE64=")
	fmt.Println(base64.StdEncoding.EncodeToString(publicKey))
	fmt.Println()
	fmt.Println("PRIVATE_KEY_BASE64=")
	fmt.Println(base64.StdEncoding.EncodeToString(privateKey))
}
