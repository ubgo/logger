package otellogger

import (
	"context"

	logger "github.com/ubgo/logger"
	"go.opentelemetry.io/otel/trace"
)

// TraceExtractor returns a logger.TraceExtractor that pulls the active W3C
// trace/span IDs out of the OTEL span in context. Wire it into the core's
// EnrichProcessor so every log correlates with its trace:
//
//	logger.NewEnrichProcessor(otellogger.TraceExtractor())
func TraceExtractor() logger.TraceExtractor {
	return func(ctx context.Context) (string, string, bool) {
		sc := trace.SpanContextFromContext(ctx)
		if !sc.IsValid() {
			return "", "", false
		}
		return sc.TraceID().String(), sc.SpanID().String(), true
	}
}
