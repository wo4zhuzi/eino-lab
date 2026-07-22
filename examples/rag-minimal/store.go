package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type memoryEntry struct {
	document *schema.Document
	vector   []float64
}

// MemoryVectorStore implements Eino's Indexer and Retriever contracts.
type MemoryVectorStore struct {
	mu               sync.RWMutex
	embedder         embedding.Embedder
	dimension        int
	defaultTopK      int
	defaultThreshold float64
	entries          map[string]memoryEntry
	order            []string
}

// NewMemoryVectorStore creates an in-process vector store.
func NewMemoryVectorStore(
	embedder embedding.Embedder,
	dimension int,
	defaultTopK int,
	defaultThreshold float64,
) (*MemoryVectorStore, error) {
	if embedder == nil {
		return nil, ErrEmbedderRequired
	}
	if dimension <= 0 {
		return nil, ErrInvalidDimension
	}
	if defaultTopK <= 0 {
		return nil, ErrInvalidTopK
	}
	if defaultThreshold < -1 || defaultThreshold > 1 {
		return nil, ErrInvalidScoreThreshold
	}
	return &MemoryVectorStore{
		embedder:         embedder,
		dimension:        dimension,
		defaultTopK:      defaultTopK,
		defaultThreshold: defaultThreshold,
		entries:          make(map[string]memoryEntry),
	}, nil
}

func (s *MemoryVectorStore) Store(
	ctx context.Context,
	docs []*schema.Document,
	opts ...indexer.Option,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, ErrNoChunks
	}

	options := indexer.GetCommonOptions(&indexer.Options{Embedding: s.embedder}, opts...)
	if options.Embedding == nil {
		return nil, ErrEmbedderRequired
	}
	texts := make([]string, len(docs))
	for i, doc := range docs {
		if doc == nil || strings.TrimSpace(doc.Content) == "" {
			return nil, fmt.Errorf("%w: empty document at index %d", ErrNoChunks, i)
		}
		texts[i] = doc.Content
	}
	vectors, err := options.Embedding.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed documents: %w", err)
	}
	if err := validateVectors(vectors, len(docs), s.dimension); err != nil {
		return nil, err
	}

	ids := make([]string, len(docs))
	entries := make([]memoryEntry, len(docs))
	for i, doc := range docs {
		copyDoc := cloneDocument(doc)
		if copyDoc.ID == "" {
			copyDoc.ID = stableID(copyDoc.Content)
		}
		ids[i] = copyDoc.ID
		entries[i] = memoryEntry{document: copyDoc, vector: append([]float64(nil), vectors[i]...)}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, id := range ids {
		if _, exists := s.entries[id]; !exists {
			s.order = append(s.order, id)
		}
		s.entries[id] = entries[i]
	}
	return ids, nil
}

func (s *MemoryVectorStore) Retrieve(
	ctx context.Context,
	query string,
	opts ...retriever.Option,
) ([]*schema.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuestion
	}

	topK := s.defaultTopK
	threshold := s.defaultThreshold
	options := retriever.GetCommonOptions(&retriever.Options{
		TopK:           &topK,
		ScoreThreshold: &threshold,
		Embedding:      s.embedder,
	}, opts...)
	if options.Embedding == nil {
		return nil, ErrEmbedderRequired
	}
	if options.TopK == nil || *options.TopK <= 0 {
		return nil, ErrInvalidTopK
	}
	if options.ScoreThreshold == nil || *options.ScoreThreshold < -1 || *options.ScoreThreshold > 1 {
		return nil, ErrInvalidScoreThreshold
	}

	vectors, err := options.Embedding.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if err := validateVectors(vectors, 1, s.dimension); err != nil {
		return nil, err
	}

	s.mu.RLock()
	entries := make([]memoryEntry, 0, len(s.order))
	for _, id := range s.order {
		entry := s.entries[id]
		entries = append(entries, memoryEntry{
			document: cloneDocument(entry.document),
			vector:   append([]float64(nil), entry.vector...),
		})
	}
	s.mu.RUnlock()

	type scoredDocument struct {
		document *schema.Document
		score    float64
	}
	scored := make([]scoredDocument, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		score := cosineSimilarity(vectors[0], entry.vector)
		if score < *options.ScoreThreshold {
			continue
		}
		scored = append(scored, scoredDocument{document: entry.document, score: score})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].document.ID < scored[j].document.ID
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > *options.TopK {
		scored = scored[:*options.TopK]
	}

	result := make([]*schema.Document, len(scored))
	for i, item := range scored {
		result[i] = item.document.WithScore(item.score)
	}
	return result, nil
}

// Count returns the number of indexed chunks.
func (s *MemoryVectorStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func stableID(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:8])
}

func cosineSimilarity(left, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for i := range left {
		dot += left[i] * right[i]
		leftNorm += left[i] * left[i]
		rightNorm += right[i] * right[i]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func cloneDocument(doc *schema.Document) *schema.Document {
	if doc == nil {
		return nil
	}
	copyDoc := &schema.Document{ID: doc.ID, Content: doc.Content}
	if doc.MetaData != nil {
		copyDoc.MetaData = make(map[string]any, len(doc.MetaData))
		for key, value := range doc.MetaData {
			switch typed := value.(type) {
			case []float64:
				copyDoc.MetaData[key] = append([]float64(nil), typed...)
			case []string:
				copyDoc.MetaData[key] = append([]string(nil), typed...)
			default:
				copyDoc.MetaData[key] = value
			}
		}
	}
	return copyDoc
}
