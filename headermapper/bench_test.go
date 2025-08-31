package headermapper

import (
	"context"
	"net/http/httptest"
	"testing"
)

func BenchmarkMetadataAnnotator(b *testing.B) {
	mapper := NewBuilder().
		AddIncomingMapping("X-User-ID", "user-id").
		AddIncomingMapping("Authorization", "auth-token").
		AddIncomingMapping("X-Request-ID", "request-id").
		AddIncomingMapping("Content-Type", "content-type").
		AddIncomingMapping("Accept", "accept").
		Build()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-User-ID", "12345")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	annotator := mapper.MetadataAnnotator()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = annotator(ctx, req)
	}
}

func BenchmarkTransformations(b *testing.B) {
	transform := ChainTransforms(
		TrimSpace,
		RemovePrefix("Bearer "),
		ToLower,
	)

	input := "  Bearer TOKEN123  "

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = transform(input)
	}
}

func BenchmarkHeaderMatcher(b *testing.B) {
	mapper := NewBuilder().
		AddIncomingMapping("X-User-ID", "user-id").
		AddIncomingMapping("Authorization", "auth-token").
		AddIncomingMapping("X-Request-ID", "request-id").
		AddIncomingMapping("Content-Type", "content-type").
		AddIncomingMapping("Accept", "accept").
		CaseSensitive(false).
		Build()

	matcher := mapper.HeaderMatcher()
	headers := []string{
		"X-User-ID",
		"authorization",
		"x-request-id",
		"Content-Type",
		"unknown-header",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, header := range headers {
			_, _ = matcher(header)
		}
	}
}

func BenchmarkBuilderPattern(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = NewBuilder().
			AddIncomingMapping("X-User-ID", "user-id").
			WithRequired(true).
			AddOutgoingMapping("response-time", "X-Response-Time").
			WithTransform(AddPrefix("Duration: ")).
			AddBidirectionalMapping("X-Request-ID", "request-id").
			SkipPaths("/health").
			Build()
	}
}
