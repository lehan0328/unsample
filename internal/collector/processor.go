package unsampleprocessor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// RoutingResult holds the outcome of processing a batch of traces.
// Debug and production spans are separated into independent trace structures
// for routing to different pipelines.
type RoutingResult struct {
	// Debug contains spans with the debug attribute set to true.
	Debug ptrace.Traces

	// Production contains all other spans.
	Production ptrace.Traces

	// DebugSpanCount is the number of spans routed to the debug pipeline.
	DebugSpanCount int

	// ProductionSpanCount is the number of spans routed to the production pipeline.
	ProductionSpanCount int

	// DroppedSpanCount is the number of debug spans dropped due to rate limiting.
	DroppedSpanCount int
}

// Processor routes spans to debug or production pipelines based on a span attribute.
//
// Design invariants (from safety guardrails):
//   - Stateless: O(1) memory per span, no trace buffering
//   - No groupbytrace: prevents OOM from fan-out
//   - Rate-limited drops are silent (nil error, never retry)
type Processor struct {
	cfg       *Config
	rateLimit *RateLimiter
	logger    *zap.Logger
}

// NewProcessor creates a new Processor with the given config.
func NewProcessor(cfg *Config, logger *zap.Logger) (*Processor, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Processor{
		cfg:       cfg,
		rateLimit: NewRateLimiter(cfg.MaxPerMinute),
		logger:    logger,
	}, nil
}

// ProcessTraces routes spans from the input traces to debug or production buckets.
// This is the core logic that would be called from ConsumeTraces in a full
// Collector processor implementation.
//
// Each span is checked independently (stateless, O(1) memory):
//   - debug.trace=true → debug pipeline (if within rate limit)
//   - debug.trace=true + rate exceeded → silently dropped
//   - anything else → production pipeline
func (p *Processor) ProcessTraces(_ context.Context, td ptrace.Traces) RoutingResult {
	result := RoutingResult{
		Debug:      ptrace.NewTraces(),
		Production: ptrace.NewTraces(),
	}

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)

			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				if p.isDebugSpan(span) {
					if p.rateLimit.Allow() {
						copySpanToTraces(result.Debug, rs, ss, span)
						result.DebugSpanCount++
						p.logger.Debug("routed span to debug pipeline",
							zap.String("span_name", span.Name()),
						)
					} else {
						result.DroppedSpanCount++
						p.logger.Warn("debug span dropped (rate limit exceeded)",
							zap.String("span_name", span.Name()),
						)
					}
				} else {
					copySpanToTraces(result.Production, rs, ss, span)
					result.ProductionSpanCount++
				}
			}
		}
	}

	return result
}

// isDebugSpan checks if a span has the debug attribute set to true.
// This is an O(1) attribute lookup — no iteration over all attributes.
func (p *Processor) isDebugSpan(span ptrace.Span) bool {
	val, exists := span.Attributes().Get(p.cfg.DebugAttribute)
	if !exists {
		return false
	}
	// Only accept boolean true — not string "true" or integer 1.
	return val.Type() == pcommon.ValueTypeBool && val.Bool()
}

// copySpanToTraces copies a span along with its resource and scope context
// into the destination Traces structure. This preserves the full telemetry
// context required for proper export.
func copySpanToTraces(dest ptrace.Traces, srcRS ptrace.ResourceSpans,
	srcSS ptrace.ScopeSpans, srcSpan ptrace.Span) {

	destRS := dest.ResourceSpans().AppendEmpty()
	srcRS.Resource().CopyTo(destRS.Resource())

	destSS := destRS.ScopeSpans().AppendEmpty()
	srcSS.Scope().CopyTo(destSS.Scope())

	destSpan := destSS.Spans().AppendEmpty()
	srcSpan.CopyTo(destSpan)
}
