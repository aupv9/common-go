package grpcserver

import (
	"crypto/rand"
	"encoding/hex"
)

// newID sinh request id ngẫu nhiên 16 byte (hex), đồng bộ format với HTTP.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(b[:])
}
