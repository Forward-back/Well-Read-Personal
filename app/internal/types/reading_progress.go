package types

import "context"

// ReadingProgressEvent is an ephemeral SSE frame for the reading workbench.
// It is never persisted; the final ReadingRun remains the source of truth.
type ReadingProgressEvent struct {
	Type           string      `json:"type"`
	Phase          string      `json:"phase,omitempty"`
	Status         string      `json:"status,omitempty"`
	SectionID      string      `json:"section_id,omitempty"`
	SectionTitle   string      `json:"section_title,omitempty"`
	SectionHeading string      `json:"section_heading,omitempty"`
	Round          int         `json:"round,omitempty"`
	Query          string      `json:"query,omitempty"`
	HitCount       int         `json:"hit_count,omitempty"`
	Content        string      `json:"content,omitempty"`
	ElapsedMS      int64       `json:"elapsed_ms,omitempty"`
	Run            *ReadingRun `json:"run,omitempty"`
}

type readingProgressSink func(ReadingProgressEvent)

type readingProgressContextKey struct{}

func WithReadingProgress(ctx context.Context, sink func(ReadingProgressEvent)) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, readingProgressContextKey{}, readingProgressSink(sink))
}

func EmitReadingProgress(ctx context.Context, event ReadingProgressEvent) {
	if sink, ok := ctx.Value(readingProgressContextKey{}).(readingProgressSink); ok {
		sink(event)
	}
}

func HasReadingProgress(ctx context.Context) bool {
	_, ok := ctx.Value(readingProgressContextKey{}).(readingProgressSink)
	return ok
}
