// Package unsample provides Go middleware for the Unsample debug tracing system.
//
// Unsample allows developers to force-capture distributed traces for specific
// HTTP requests by injecting a signed debug token. This package provides the
// SDK interceptor that runs in each microservice to:
//
//  1. Check for the debug token in W3C baggage (O(1), zero-alloc hot path)
//  2. Verify the HMAC signature
//  3. Set debug.trace=true on the current span
//  4. OTel propagators automatically forward baggage to downstream calls
//
// Usage:
//
//	import unsample "github.com/unsample/unsample/sdk/go"
//
//	router.Use(unsample.Middleware(unsample.Config{
//	    Secret: os.Getenv("UNSAMPLE_SECRET"),
//	}))
package unsample

import "time"

// Config configures the Unsample middleware.
type Config struct {
	// Secret is the shared HMAC secret for token verification.
	// Must match the secret used by the CLI and all other services.
	// Required — middleware is a no-op if empty.
	Secret string

	// TokenMaxAge is the maximum age of a valid token.
	// Tokens older than this are rejected to prevent replay attacks.
	// Default: 2 hours.
	TokenMaxAge time.Duration

	// BaggageKey is the baggage member key to read the debug token from.
	// Default: "unsample-debug".
	BaggageKey string

	// AttributeKey is the span attribute key to set on debug spans.
	// Default: "debug.trace".
	AttributeKey string
}

// defaults fills in zero-value fields with sensible defaults.
func (c *Config) defaults() {
	if c.TokenMaxAge == 0 {
		c.TokenMaxAge = 2 * time.Hour
	}
	if c.BaggageKey == "" {
		c.BaggageKey = "unsample-debug"
	}
	if c.AttributeKey == "" {
		c.AttributeKey = "debug.trace"
	}
}
