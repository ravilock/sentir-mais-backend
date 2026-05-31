package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func New(prefix string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}

	return prefix + "_" + hex.EncodeToString(buf), nil
}
