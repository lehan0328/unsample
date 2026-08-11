package unsample

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

// Middleware returns an HTTP middleware that enables Unsample debug tracing.
//
// The middleware has two code paths:
//
// Hot path (99.99% of requests):
//
//	Read baggage → no debug token → call next handler immediately.
//	This is O(1), zero-allocation, and sub-microsecond.
//
// Cold path (debug requests only):
//
//	Verify HMAC token → set debug.trace=true span attribute → call next handler.
//	Baggage is automatically propagated to downstream calls by OTel propagators.
//
// If the secret is empty, the middleware is a no-op (all requests pass through).
// If the token is invalid or expired, the request is treated as normal (not debug).
func Middleware(cfg Config) func(http.Handler) http.Handler {
	cfg.defaults()

	return func(next http.Handler) http.Handler {
		// No secret configured — middleware is a no-op.
		if cfg.Secret == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ─── HOT PATH ─────────────────────────────
			// O(1) check, zero allocation, sub-microsecond.
			// 99.99% of requests exit here.
			token := baggage.FromContext(r.Context()).Member(cfg.BaggageKey).Value()
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			// ─── COLD PATH (debug request) ────────────
			// Verify HMAC token. Invalid/expired tokens are silently ignored.
			if !verifyToken(token, cfg.Secret, cfg.TokenMaxAge) {
				next.ServeHTTP(w, r)
				return
			}

			// Token is valid — mark this span as debug.
			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(attribute.Bool(cfg.AttributeKey, true))

			next.ServeHTTP(w, r)
		})
	}
}
