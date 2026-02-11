package files

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diane-assistant/diane/internal/db"
)

// EmbeddingService manages embeddings using configured providers
type EmbeddingService struct {
	filesDB    *DB
	mainDB     *db.DB
	client     *EmbeddingClient
	providerID int64
	service    string // e.g., "vertex_ai"
	model      string // e.g., "text-embedding-005"
}

// NewEmbeddingService creates a new embedding service
// It uses the default embedding provider from the database
func NewEmbeddingService(ctx context.Context, filesDB *DB, mainDB *db.DB) (*EmbeddingService, error) {
	provider, err := mainDB.GetDefaultProvider(db.ProviderTypeEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding provider: %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("no embedding provider configured. Add one via the API at /providers")
	}

	return NewEmbeddingServiceWithProvider(ctx, filesDB, mainDB, provider)
}

// NewEmbeddingServiceWithProvider creates an embedding service with a specific provider
func NewEmbeddingServiceWithProvider(ctx context.Context, filesDB *DB, mainDB *db.DB, provider *db.Provider) (*EmbeddingService, error) {
	if !provider.Enabled {
		return nil, fmt.Errorf("provider %s is disabled", provider.Name)
	}

	var client *EmbeddingClient
	var err error

	switch provider.Service {
	case "vertex_ai":
		client, err = createVertexAIClient(ctx, provider)
	case "openai":
		// TODO: Implement OpenAI embeddings client
		return nil, fmt.Errorf("OpenAI embeddings not yet implemented")
	case "ollama":
		// TODO: Implement Ollama embeddings client
		return nil, fmt.Errorf("Ollama embeddings not yet implemented")
	default:
		return nil, fmt.Errorf("unknown embedding service: %s", provider.Service)
	}

	if err != nil {
		return nil, err
	}

	return &EmbeddingService{
		filesDB:    filesDB,
		mainDB:     mainDB,
		client:     client,
		providerID: provider.ID,
		service:    provider.Service,
		model:      provider.GetConfigString("model"),
	}, nil
}

func createVertexAIClient(ctx context.Context, provider *db.Provider) (*EmbeddingClient, error) {
	projectID := provider.GetConfigString("project_id")
	if projectID == "" {
		return nil, fmt.Errorf("vertex_ai provider requires project_id in config")
	}

	location := provider.GetConfigString("location")
	if location == "" {
		location = "us-central1"
	}

	model := provider.GetConfigString("model")
	if model == "" {
		model = ModelTextEmbedding005
	}

	account := provider.GetAuthString("oauth_account")
	if account == "" {
		account = "default"
	}

	client, err := NewEmbeddingClient(ctx, projectID, location, account)
	if err != nil {
		return nil, err
	}

	return client.WithModel(model), nil
}

// EmbedFile generates and stores an embedding for a file
func (s *EmbeddingService) EmbedFile(ctx context.Context, file *File) error {
	if file.ContentText == "" && file.ContentPreview == "" {
		return fmt.Errorf("file has no content to embed")
	}

	// Prepare text for embedding
	text := PrepareTextForEmbedding(file)

	// Generate embedding with usage tracking
	embedding, usage, err := s.client.EmbedTextWithUsage(ctx, text, TaskTypeRetrievalDocument)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Record usage
	if usage != nil && usage.TotalTokens > 0 {
		s.recordUsage(usage.TotalTokens)
	}

	// Get provider info for metadata
	provider, _ := s.mainDB.GetProvider(s.providerID)
	modelName := ""
	if provider != nil {
		modelName = fmt.Sprintf("%s/%s", provider.Service, provider.GetConfigString("model"))
	}

	// Store embedding
	fe := &FileEmbedding{
		FileID:    file.ID,
		Embedding: embedding,
		Model:     modelName,
	}

	return s.filesDB.UpsertEmbedding(fe)
}

// EmbedFiles generates embeddings for multiple files (batch)
func (s *EmbeddingService) EmbedFiles(ctx context.Context, files []*File) error {
	if len(files) == 0 {
		return nil
	}

	// Prepare texts
	texts := make([]string, len(files))
	for i, f := range files {
		texts[i] = PrepareTextForEmbedding(f)
	}

	// Generate embeddings in batch with usage tracking
	embeddings, usage, err := s.client.EmbedTextsBatched(ctx, texts, TaskTypeRetrievalDocument, 100)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Record usage
	if usage != nil && usage.TotalTokens > 0 {
		s.recordUsage(usage.TotalTokens)
	}

	// Get model name
	provider, _ := s.mainDB.GetProvider(s.providerID)
	modelName := ""
	if provider != nil {
		modelName = fmt.Sprintf("%s/%s", provider.Service, provider.GetConfigString("model"))
	}

	// Store embeddings
	for i, file := range files {
		fe := &FileEmbedding{
			FileID:    file.ID,
			Embedding: embeddings[i],
			Model:     modelName,
		}
		if err := s.filesDB.UpsertEmbedding(fe); err != nil {
			return fmt.Errorf("failed to store embedding for file %d: %w", file.ID, err)
		}
	}

	return nil
}

// SearchSimilar finds files similar to the given query text
func (s *EmbeddingService) SearchSimilar(ctx context.Context, query string, limit int) ([]*VectorSearchResult, error) {
	// Generate query embedding with usage tracking
	embedding, usage, err := s.client.EmbedTextWithUsage(ctx, query, TaskTypeRetrievalQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Record usage
	if usage != nil && usage.TotalTokens > 0 {
		s.recordUsage(usage.TotalTokens)
	}

	// Search
	return s.filesDB.VectorSearchWithFiles(embedding, limit)
}

// ProcessPendingEmbeddings generates embeddings for files that don't have them yet
func (s *EmbeddingService) ProcessPendingEmbeddings(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	// Get files without embeddings
	files, err := s.filesDB.GetFilesWithoutEmbeddings(batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get files without embeddings: %w", err)
	}

	if len(files) == 0 {
		return 0, nil
	}

	// Generate embeddings
	if err := s.EmbedFiles(ctx, files); err != nil {
		return 0, err
	}

	return len(files), nil
}

// recordUsage records embedding token usage to the database
func (s *EmbeddingService) recordUsage(inputTokens int) {
	if s.mainDB == nil {
		return
	}

	// Get model name - default if not set
	model := s.model
	if model == "" {
		model = ModelTextEmbedding005
	}

	// For embeddings, we only have input tokens (no output)
	usage := &db.Usage{
		ProviderID:   s.providerID,
		Service:      s.service,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: 0,
		CachedTokens: 0,
		// Cost will be calculated when we have embedding pricing in models.dev
		// For now, Vertex AI text-embedding-005 is ~$0.000025 per 1K characters
		Cost: 0,
	}

	if _, err := s.mainDB.RecordUsage(usage); err != nil {
		slog.Warn("Failed to record embedding usage", "error", err, "provider_id", s.providerID)
	}
}
