package unsample

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// UnaryServerInterceptor returns a gRPC server interceptor that enables
// Unsample debug tracing for unary RPCs.
//
// The interceptor has the same hot/cold path design as the HTTP middleware:
//
// Hot path (99.99% of RPCs):
//
//	Read baggage from context → no debug token → call handler immediately.
//	O(1), zero-allocation, sub-microsecond.
//
// Cold path (debug RPCs only):
//
//	Verify HMAC token → set debug.trace=true span attribute → call handler.
//
// Prerequisites:
//   - OTel propagators must be configured to include baggage propagation.
//   - Use otelgrpc.NewServerHandler() as a StatsHandler to enable context propagation.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StatsHandler(otelgrpc.NewServerHandler()),
//	    grpc.ChainUnaryInterceptor(
//	        unsample.UnaryServerInterceptor(unsample.Config{
//	            Secret: os.Getenv("UNSAMPLE_SECRET"),
//	        }),
//	    ),
//	)
func UnaryServerInterceptor(cfg Config) grpc.UnaryServerInterceptor {
	cfg.defaults()

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// No secret configured — interceptor is a no-op.
		if cfg.Secret == "" {
			return handler(ctx, req)
		}

		// ─── HOT PATH ─────────────────────────────
		// O(1) check, zero allocation, sub-microsecond.
		// 99.99% of RPCs exit here.
		token := baggage.FromContext(ctx).Member(cfg.BaggageKey).Value()
		if token == "" {
			return handler(ctx, req)
		}

		// ─── COLD PATH (debug RPC) ────────────────
		// Verify HMAC token. Invalid/expired tokens are silently ignored.
		if !verifyToken(token, cfg.Secret, cfg.TokenMaxAge) {
			return handler(ctx, req)
		}

		// Token is valid — mark this span as debug.
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.Bool(cfg.AttributeKey, true))

		return handler(ctx, req)
	}
}

// StreamServerInterceptor returns a gRPC server interceptor that enables
// Unsample debug tracing for streaming RPCs.
//
// Same hot/cold path design as UnaryServerInterceptor.
func StreamServerInterceptor(cfg Config) grpc.StreamServerInterceptor {
	cfg.defaults()

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// No secret configured — interceptor is a no-op.
		if cfg.Secret == "" {
			return handler(srv, ss)
		}

		// ─── HOT PATH ─────────────────────────────
		token := baggage.FromContext(ss.Context()).Member(cfg.BaggageKey).Value()
		if token == "" {
			return handler(srv, ss)
		}

		// ─── COLD PATH (debug stream) ─────────────
		if !verifyToken(token, cfg.Secret, cfg.TokenMaxAge) {
			return handler(srv, ss)
		}

		span := trace.SpanFromContext(ss.Context())
		span.SetAttributes(attribute.Bool(cfg.AttributeKey, true))

		return handler(srv, ss)
	}
}
