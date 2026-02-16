// Package pairing provides time-based pairing codes for connecting client apps
// to a Diane server without manually sharing API keys.
//
// The pairing code is derived from the API key and the current time window using
// HMAC-SHA256, producing a 6-digit numeric code valid for 30 seconds. Both the
// current and previous time windows are accepted to handle clock skew and typing
// delay (effective validity: ~60 seconds).
//
// Flow:
//  1. User runs "diane pair" on the server -> displays a 6-digit code
//  2. Client app sends POST /pair with {"code": "123456"}
//  3. Server validates the code and returns {"api_key": "..."}
//  4. Client stores the API key for future requests
package pairing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	// TimeWindow is the duration for which a pairing code is valid.
	TimeWindow = 30 * time.Second

	// CodeDigits is the number of digits in the pairing code.
	CodeDigits = 6

	// codeMod is 10^CodeDigits, used to truncate the HMAC output.
	codeMod = 1_000_000

	// MaxAttempts is the maximum number of failed pairing attempts per IP
	// within the rate limit window before requests are rejected.
	MaxAttempts = 5

	// RateLimitWindow is the time window for rate limiting.
	RateLimitWindow = 1 * time.Minute
)

// GenerateCode produces a 6-digit pairing code for the current time window.
// The code is derived from the API key and the current time step using HMAC-SHA256.
func GenerateCode(apiKey string) string {
	step := currentTimeStep()
	return codeForStep(apiKey, step)
}

// ValidateCode checks if the provided code matches the current or previous time window.
// Returns true if the code is valid.
func ValidateCode(apiKey string, code string) bool {
	step := currentTimeStep()

	// Accept current window and previous window (~60s effective validity)
	if code == codeForStep(apiKey, step) {
		return true
	}
	if code == codeForStep(apiKey, step-1) {
		return true
	}
	return false
}

// TimeRemaining returns the number of seconds remaining in the current time window.
func TimeRemaining() int {
	elapsed := time.Now().Unix() % int64(TimeWindow.Seconds())
	return int(TimeWindow.Seconds()) - int(elapsed)
}

// codeForStep computes the pairing code for a given time step.
func codeForStep(apiKey string, step int64) string {
	// Create HMAC-SHA256 with the API key as the key
	mac := hmac.New(sha256.New, []byte(apiKey))

	// Write the time step as big-endian uint64
	stepBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(stepBytes, uint64(step))
	mac.Write(stepBytes)

	hash := mac.Sum(nil)

	// Dynamic truncation (similar to TOTP RFC 6238)
	// Use the last nibble of the hash to determine the offset
	offset := hash[len(hash)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	// Reduce to CodeDigits digits
	code := truncated % codeMod

	return fmt.Sprintf("%0*d", CodeDigits, code)
}

// currentTimeStep returns the current time step (seconds since epoch / window size).
func currentTimeStep() int64 {
	return time.Now().Unix() / int64(TimeWindow.Seconds())
}

// FormatCode formats a 6-digit code as "XXX XXX" for easy reading.
func FormatCode(code string) string {
	if len(code) == 6 {
		return code[:3] + " " + code[3:]
	}
	return code
}

// RateLimiter tracks failed pairing attempts per IP address.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

// NewRateLimiter creates a new rate limiter for pairing attempts.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
	}
}

// Allow checks if the given IP is allowed to attempt pairing.
// Returns true if under the rate limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-RateLimitWindow)

	// Remove expired entries
	var recent []time.Time
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	rl.attempts[ip] = recent

	return len(recent) < MaxAttempts
}

// Record records a failed attempt from the given IP.
func (rl *RateLimiter) Record(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.attempts[ip] = append(rl.attempts[ip], time.Now())
}

// ClientIP extracts the client IP from an HTTP request,
// checking X-Forwarded-For and X-Real-IP headers first.
func ClientIP(r *http.Request) string {
	// Check forwarded headers
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
