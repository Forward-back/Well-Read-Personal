package embedding

import "testing"

func TestEmbeddingDimensionsParamRequiresPositiveDimension(t *testing.T) {
	e := &OpenAIEmbedder{dimensions: 0}
	e.SetSupportsDimensionOverride(true)
	if e.supportsDimensionsParam() {
		t.Fatal("expected dimensions param to be omitted when configured dimension is zero")
	}
}
