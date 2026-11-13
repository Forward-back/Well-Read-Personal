package service

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"strings"

	"gaze/internal/logger"
	"gaze/internal/models/vlm"
)

const (
	// maxOCRTiles bounds the tiled retry so a pathological image cannot fan
	// out into unbounded VLM calls.
	maxOCRTiles = 4
	// minOCRTileHeight is the smallest slice worth sending; below it the
	// slice costs a request but carries little text.
	minOCRTileHeight = 320
	// ocrTileOverlapPercent keeps text that straddles a cut readable in at
	// least one tile.
	ocrTileOverlapPercent = 8
)

// ocrImageWithTiling runs OCR once on the whole image. When the model stops on
// its token budget (finish_reason=length) it retries with vertical slices and
// merges the text, so dense pages lose content instead of failing outright.
func ocrImageWithTiling(ctx context.Context, model vlm.VLM, imgBytes []byte, prompt string) (string, error) {
	text, err := model.Predict(ctx, [][]byte{imgBytes}, prompt)
	text = strings.TrimSpace(text)
	if err == nil && text != "" {
		return text, nil
	}
	if err == nil || !strings.Contains(err.Error(), "truncated") {
		return text, err
	}

	tiles, splitErr := splitImageTiles(imgBytes, maxOCRTiles, minOCRTileHeight, ocrTileOverlapPercent)
	if splitErr != nil || len(tiles) < 2 {
		// Very small or oddly shaped images cannot be sliced. Retry once with
		// a compact-output instruction so the transcript fits the budget.
		compactPrompt := prompt + "\n\n【重要】输出超过单次长度限制，请只保留关键文字与表格，省略解释与重复内容。"
		compactText, compactErr := model.Predict(ctx, [][]byte{imgBytes}, compactPrompt)
		compactText = strings.TrimSpace(compactText)
		if compactErr == nil && compactText != "" {
			logger.Infof(ctx, "[ImageMultimodal] OCR truncated, compact retry succeeded")
			return compactText, nil
		}
		if compactErr != nil {
			return "", compactErr
		}
		return "", err
	}
	logger.Infof(ctx, "[ImageMultimodal] OCR truncated, retrying with %d vertical tiles", len(tiles))

	parts := make([]string, 0, len(tiles))
	var tileErr error
	for _, tile := range tiles {
		tileText, predictErr := model.Predict(ctx, [][]byte{tile}, prompt)
		if predictErr != nil {
			if tileErr == nil {
				tileErr = predictErr
			}
			continue
		}
		tileText = strings.TrimSpace(tileText)
		if tileText == "" || tileText == "无文字内容" {
			continue
		}
		parts = append(parts, tileText)
	}
	merged := mergeOCRPartText(parts)
	if merged == "" {
		if tileErr != nil {
			return "", tileErr
		}
		return "", err
	}
	return merged, nil
}

// splitImageTiles slices an image along its longer axis with a small overlap.
// Tall pages are cut into horizontal bands; wide, short figures (tables,
// screenshots) are cut into vertical strips so both shapes can be transcribed
// without exceeding the per-request output budget. Tiles are re-encoded as PNG
// so every OpenAI-compatible VLM accepts them.
func splitImageTiles(data []byte, maxTiles, minTileSize, overlapPercent int) ([][]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	vertical := height >= minTileSize*2
	horizontal := !vertical && width >= minTileSize*2
	if !vertical && !horizontal {
		return nil, nil
	}
	axisLen := height
	if horizontal {
		axisLen = width
	}
	tilesCount := (axisLen + minTileSize - 1) / minTileSize
	if tilesCount < 2 {
		tilesCount = 2
	}
	if tilesCount > maxTiles {
		tilesCount = maxTiles
	}
	base := axisLen / tilesCount
	if base < 1 {
		return nil, nil
	}
	overlap := base * overlapPercent / 100
	if overlap < 16 {
		overlap = 16
	}

	tiles := make([][]byte, 0, tilesCount)
	for i := 0; i < tilesCount; i++ {
		start := i*base - overlap
		if start < 0 {
			start = 0
		}
		end := (i+1)*base + overlap
		if i == tilesCount-1 {
			end = axisLen
		}
		if end > axisLen {
			end = axisLen
		}
		if end <= start {
			continue
		}
		var rect image.Rectangle
		if horizontal {
			rect = image.Rect(bounds.Min.X+start, bounds.Min.Y, bounds.Min.X+end, bounds.Max.Y)
		} else {
			rect = image.Rect(bounds.Min.X, bounds.Min.Y+start, bounds.Max.X, bounds.Min.Y+end)
		}
		sub := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
		draw.Draw(sub, sub.Bounds(), img, rect.Min, draw.Src)
		var buf bytes.Buffer
		if err := png.Encode(&buf, sub); err != nil {
			return nil, err
		}
		tiles = append(tiles, buf.Bytes())
	}
	return tiles, nil
}

// mergeOCRPartText joins tile transcripts and removes the duplicated lines the
// overlap window produces at each boundary. Only the previous tail is compared,
// so legitimate repeated rows elsewhere are kept.
func mergeOCRPartText(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	merged := strings.TrimSpace(parts[0])
	for _, next := range parts[1:] {
		tail := lastNonEmptyLines(merged, 6)
		lines := strings.Split(next, "\n")
		skip := 0
		for skip < len(lines) {
			line := strings.TrimSpace(lines[skip])
			if line == "" {
				skip++
				continue
			}
			if _, dup := tail[line]; !dup {
				break
			}
			skip++
		}
		rest := strings.TrimSpace(strings.Join(lines[skip:], "\n"))
		if rest == "" {
			continue
		}
		merged = merged + "\n" + rest
	}
	return strings.TrimSpace(merged)
}

func lastNonEmptyLines(text string, limit int) map[string]struct{} {
	out := make(map[string]struct{}, limit)
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			out[line] = struct{}{}
		}
	}
	return out
}
