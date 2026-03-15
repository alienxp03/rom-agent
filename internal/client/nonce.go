package client

import (
	"crypto/sha1"
	"fmt"
	"time"

	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
)

// NonceGen generates security nonces for game protocol
type NonceGen struct {
	currentIndex int64
	currentTime  int64
	serverTime   int64
	deltaTime    int64
	startTime    time.Time
}

// NewNonceGen creates a new nonce generator
func NewNonceGen() *NonceGen {
	return &NonceGen{
		currentIndex: 1,
		currentTime:  0,
		serverTime:   0,
		deltaTime:    0,
		startTime:    time.Now(),
	}
}

// Update updates the time and indices before each network request
func (ng *NonceGen) Update() {
	if ng.serverTime > 0 {
		ng.serverTime = ng.GetServerTime()
	}

	now := ng.serverTime
	if ng.serverTime > 0 {
		now = now / 1000
	} else {
		now = 0
	}

	delta := now - ng.currentTime
	if delta > 0 {
		ng.currentIndex = 1
	} else if delta < 0 {
		now = ng.currentTime + 1
		ng.currentIndex = 1
	} else {
		ng.currentIndex++
	}
}

// SetServerTime sets the server time in milliseconds
func (ng *NonceGen) SetServerTime(serverTime int64) {
	ng.serverTime = serverTime
	ng.deltaTime = serverTime - ng.getClientTime()
}

// getClientTime returns the time since the process started, in milliseconds
func (ng *NonceGen) getClientTime() int64 {
	return time.Since(ng.startTime).Milliseconds()
}

// GetServerTime returns the server time (computed from client time and delta), in milliseconds
func (ng *NonceGen) GetServerTime() int64 {
	return ng.getClientTime() + ng.deltaTime
}

// getSha1 returns the SHA-1 hash of the given string as hex
func (ng *NonceGen) getSha1(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// GetNonce creates and returns a nonce
func (ng *NonceGen) GetNonce() *pb.Nonce {
	sign := fmt.Sprintf("%d_%d_!^ro&", ng.currentTime, ng.currentIndex)
	sign = ng.getSha1(sign)

	index := uint32(ng.currentIndex)
	timestamp := uint32(ng.currentTime)

	return &pb.Nonce{
		Index:     &index,
		Timestamp: &timestamp,
		Sign:      &sign,
	}
}
