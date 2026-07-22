package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
)

// HashingEmbedder maps character n-grams into a fixed local vector space.
// It is deterministic and offline, but it is not a production semantic model.
type HashingEmbedder struct {
	dimension int
}

// NewHashingEmbedder creates a deterministic fixed-dimension embedder.
func NewHashingEmbedder(dimension int) (*HashingEmbedder, error) {
	if dimension <= 0 {
		return nil, ErrInvalidDimension
	}
	return &HashingEmbedder{dimension: dimension}, nil
}

func (e *HashingEmbedder) EmbedStrings(
	ctx context.Context,
	texts []string,
	_ ...embedding.Option,
) (vectors [][]float64, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, e.GetType(), components.ComponentOfEmbedding)
	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{Texts: texts})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
			return
		}
		callbacks.OnEnd(ctx, &embedding.CallbackOutput{Embeddings: vectors})
	}()

	vectors = make([][]float64, len(texts))
	for i, text := range texts {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		vectors[i] = e.embed(text)
	}
	return vectors, nil
}

// GetType supplies a stable component name for callbacks and graph inspection.
func (e *HashingEmbedder) GetType() string {
	return "Hashing"
}

// IsCallbacksEnabled tells Eino that this component emits its own callbacks.
func (e *HashingEmbedder) IsCallbacksEnabled() bool {
	return true
}

func (e *HashingEmbedder) embed(text string) []float64 {
	vector := make([]float64, e.dimension)
	for _, segment := range normalizedSegments(text) {
		for size := 1; size <= 3 && size <= len(segment); size++ {
			weight := float64(size)
			for start := 0; start+size <= len(segment); start++ {
				feature := string(segment[start : start+size])
				bucket := hashFeature(feature) % uint64(e.dimension)
				vector[bucket] += weight
			}
		}
	}

	var normSquared float64
	for _, value := range vector {
		normSquared += value * value
	}
	if normSquared == 0 {
		return vector
	}
	norm := math.Sqrt(normSquared)
	for i := range vector {
		vector[i] /= norm
	}
	return vector
}

func normalizedSegments(text string) [][]rune {
	var segments [][]rune
	current := make([]rune, 0, len(text))
	flush := func() {
		if len(current) == 0 {
			return
		}
		segments = append(segments, append([]rune(nil), current...))
		current = current[:0]
	}

	for _, r := range []rune(strings.ToLower(text)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current = append(current, r)
			continue
		}
		flush()
	}
	flush()
	return segments
}

func hashFeature(feature string) uint64 {
	const (
		offset = uint64(14695981039346656037)
		prime  = uint64(1099511628211)
	)
	hash := offset
	for i := 0; i < len(feature); i++ {
		hash ^= uint64(feature[i])
		hash *= prime
	}
	return hash
}

func validateVectors(vectors [][]float64, expectedCount, expectedDimension int) error {
	if len(vectors) != expectedCount {
		return fmt.Errorf("%w: got %d, want %d", ErrEmbeddingCountMismatch, len(vectors), expectedCount)
	}
	for i, vector := range vectors {
		if len(vector) != expectedDimension {
			return fmt.Errorf("%w at index %d: got %d, want %d", ErrEmbeddingDimension, i, len(vector), expectedDimension)
		}
		var normSquared float64
		for _, value := range vector {
			normSquared += value * value
		}
		if normSquared == 0 {
			return fmt.Errorf("%w at index %d", ErrZeroEmbedding, i)
		}
	}
	return nil
}
