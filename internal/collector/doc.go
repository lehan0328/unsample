// Package unsampleprocessor implements an OTel Collector processor that routes
// spans to debug or production pipelines based on the debug.trace span attribute.
//
// The processor is stateless — it makes routing decisions per-span with O(1) memory.
// No trace buffering (groupbytrace) is used, which prevents OOM crashes under fan-out.
//
// Debug spans that exceed the rate limit are silently dropped (never retried).
package unsampleprocessor
