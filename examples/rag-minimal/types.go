package main

import "errors"

const (
	metaSource  = "source"
	metaHeading = "heading"
	metaChunkID = "chunk_id"
)

var (
	ErrEmptySource            = errors.New("source path is empty")
	ErrEmptyQuestion          = errors.New("question is empty")
	ErrNoChunks               = errors.New("no non-empty chunks to index")
	ErrEmbedderRequired       = errors.New("embedder is required")
	ErrInvalidDimension       = errors.New("embedding dimension must be positive")
	ErrEmbeddingCountMismatch = errors.New("embedding count does not match input count")
	ErrEmbeddingDimension     = errors.New("embedding dimension mismatch")
	ErrZeroEmbedding          = errors.New("embedding vector has zero norm")
	ErrInvalidTopK            = errors.New("top k must be positive")
	ErrInvalidScoreThreshold  = errors.New("score threshold must be between -1 and 1")
	ErrEmptyModelResponse     = errors.New("chat model returned an empty response")
	ErrDependencyUnavailable  = errors.New("dependency unavailable")
)

// Config controls the deterministic offline RAG pipeline.
type Config struct {
	EmbeddingDimension int
	TopK               int
	ScoreThreshold     float64
}

// DefaultConfig returns settings suitable for the bundled learning notes.
func DefaultConfig() Config {
	return Config{
		EmbeddingDimension: 512,
		TopK:               3,
		ScoreThreshold:     0.08,
	}
}

// IndexReport summarizes one completed file indexing run.
type IndexReport struct {
	Source    string
	Documents int
	Chunks    int
	ChunkIDs  []string
}

// QueryRequest is the input to the query graph.
type QueryRequest struct {
	Question string
}

// RetrievedChunk is a ranked document returned by the Retriever.
type RetrievedChunk struct {
	Rank    int
	Score   float64
	Source  string
	Heading string
	ChunkID string
	Content string
}

// QueryResult contains the final answer and the exact retrieved evidence.
type QueryResult struct {
	Question   string
	Answer     string
	Retrieved  []RetrievedChunk
	NoEvidence bool
}
