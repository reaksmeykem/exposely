package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Payload struct {
	Owner     string `json:"owner"`
	Email     string `json:"email,omitempty"`
	Plan      string `json:"plan,omitempty"`
	IsAdmin   bool   `json:"is_admin"`
	ExpiresAt string `json:"expires_at,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	IssuedAt  string `json:"issued_at,omitempty"`
}

func VerifyToken(token string, publicKey ed25519.PublicKey, deviceID string, now time.Time) (Payload, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Payload{}, errors.New("license token is required")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return Payload{}, errors.New("license public key is not configured")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Payload{}, errors.New("license token format is invalid")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Payload{}, fmt.Errorf("license payload is invalid: %w", err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Payload{}, fmt.Errorf("license signature is invalid: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return Payload{}, errors.New("license signature length is invalid")
	}
	if !ed25519.Verify(publicKey, payloadBytes, signature) {
		return Payload{}, errors.New("license signature verification failed")
	}

	var payload Payload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return Payload{}, fmt.Errorf("license payload JSON is invalid: %w", err)
	}

	if strings.TrimSpace(payload.ExpiresAt) != "" {
		expiresAt, err := time.Parse(time.RFC3339, payload.ExpiresAt)
		if err != nil {
			return Payload{}, errors.New("license expiry date is invalid")
		}
		if now.After(expiresAt) {
			return Payload{}, errors.New("license has expired")
		}
	}

	if expectedDeviceID := strings.TrimSpace(payload.DeviceID); expectedDeviceID != "" {
		if !strings.EqualFold(expectedDeviceID, strings.TrimSpace(deviceID)) {
			return Payload{}, errors.New("license is not valid for this device")
		}
	}

	return payload, nil
}
