package service

import (
	"fmt"
	"testing"

	"gaze/internal/infrastructure/docparser"
	"github.com/stretchr/testify/require"
)

func TestSelectMultimodalImagesDedupesAndCaps(t *testing.T) {
	images := []docparser.StoredImage{
		{ServingURL: "resource://a"},
		{ServingURL: "resource://a"},
		{ServingURL: "  "},
	}
	for i := 0; i < maxMultimodalImagesPerDocument+5; i++ {
		images = append(images, docparser.StoredImage{ServingURL: fmt.Sprintf("resource://img-%d", i)})
	}
	selected, skipped := selectMultimodalImages(images)

	require.Len(t, selected, maxMultimodalImagesPerDocument)
	require.Equal(t, "resource://a", selected[0].ServingURL)
	require.Equal(t, len(images)-len(selected), skipped)
	for i, img := range selected {
		for _, other := range selected[i+1:] {
			require.NotEqual(t, img.ServingURL, other.ServingURL)
		}
	}
}

func TestSelectMultimodalImagesKeepsEverythingUnderBudget(t *testing.T) {
	images := []docparser.StoredImage{
		{ServingURL: "resource://a"},
		{ServingURL: "resource://b"},
	}
	selected, skipped := selectMultimodalImages(images)
	require.Len(t, selected, 2)
	require.Zero(t, skipped)
}
