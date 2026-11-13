package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type ocrTilingVLMStub struct {
	calls       int
	truncate    bool
	tileTexts   []string
	failWith    error
	firstResult string
	prompts     []string
}

func (s *ocrTilingVLMStub) Predict(_ context.Context, _ [][]byte, prompt string) (string, error) {
	s.calls++
	s.prompts = append(s.prompts, prompt)
	if s.calls == 1 {
		if s.failWith != nil {
			return "", s.failWith
		}
		if s.truncate {
			return "", errors.New("OpenAI VLM returned no content: completion truncated at 5000 tokens (finish_reason=length)")
		}
		return s.firstResult, nil
	}
	idx := s.calls - 2
	if idx < len(s.tileTexts) {
		return s.tileTexts[idx], nil
	}
	return "", nil
}

func (s *ocrTilingVLMStub) GetModelName() string { return "stub-vlm" }
func (s *ocrTilingVLMStub) GetModelID() string   { return "stub-vlm-id" }

func tallPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestOCRImageWithTilingKeepsWholeImageResult(t *testing.T) {
	model := &ocrTilingVLMStub{firstResult: "普通 OCR 文本"}
	text, err := ocrImageWithTiling(context.Background(), model, tallPNG(t, 200, 900), "ocr")
	require.NoError(t, err)
	require.Equal(t, "普通 OCR 文本", text)
	require.Equal(t, 1, model.calls)
}

func TestOCRImageWithTilingRetriesTruncatedImage(t *testing.T) {
	model := &ocrTilingVLMStub{
		truncate: true,
		tileTexts: []string{
			"alpha\nbeta",
			"beta\ngamma",
			"gamma\ndelta",
			"delta\nepsilon",
		},
	}
	text, err := ocrImageWithTiling(context.Background(), model, tallPNG(t, 200, 1000), "ocr")
	require.NoError(t, err)
	require.Equal(t, 5, model.calls, "one full-image call plus four tiles")
	for _, want := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		require.Contains(t, text, want)
	}
	require.Equal(t, 1, strings.Count(text, "beta"), "overlap line must not duplicate")
	require.Equal(t, 1, strings.Count(text, "delta"), "overlap line must not duplicate")
}

func TestOCRImageWithTilingSplitsWideImageHorizontally(t *testing.T) {
	model := &ocrTilingVLMStub{
		truncate:  true,
		tileTexts: []string{"left", "mid-left", "mid-right", "right"},
	}
	text, err := ocrImageWithTiling(context.Background(), model, tallPNG(t, 1400, 400), "ocr")
	require.NoError(t, err)
	require.Equal(t, 5, model.calls, "wide image is cut into four vertical strips")
	for _, want := range []string{"left", "mid-left", "mid-right", "right"} {
		require.Contains(t, text, want)
	}
}

func TestOCRImageWithTilingFallsBackToCompactPrompt(t *testing.T) {
	model := &ocrTilingVLMStub{
		truncate:  true,
		tileTexts: []string{"compacted transcript"},
	}
	text, err := ocrImageWithTiling(context.Background(), model, tallPNG(t, 200, 200), "ocr")
	require.NoError(t, err)
	require.Equal(t, "compacted transcript", text)
	require.Equal(t, 2, model.calls, "unsplittable image retries once with a compact prompt")
	require.Contains(t, model.prompts[1], "只保留关键文字")
}

func TestOCRImageWithTilingReturnsNonTruncationError(t *testing.T) {
	model := &ocrTilingVLMStub{failWith: errors.New("connection reset")}
	_, err := ocrImageWithTiling(context.Background(), model, tallPNG(t, 200, 1000), "ocr")
	require.Error(t, err)
	require.Equal(t, 1, model.calls, "non-truncation errors must not fan out into tiles")
}
