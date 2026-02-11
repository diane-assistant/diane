package files

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/diane-assistant/diane/mcp/tools/google/auth"
	"golang.org/x/oauth2"
)

// VertexAI embedding models
const (
	ModelTextEmbedding005 = "text-embedding-005"
	ModelTextEmbedding004 = "text-embedding-004"
)

// VertexAIScope is the OAuth scope needed for Vertex AI
const VertexAIScope = "https://www.googleapis.com/auth/cloud-platform"

// EmbeddingClient provides access to Vertex AI text embeddings
type EmbeddingClient struct {
	projectID  string
	location   string
	model      string
	httpClient *http.Client
	account    string
}

// NewEmbeddingClient creates a new Vertex AI embedding client
func NewEmbeddingClient(ctx context.Context, projectID, location, account string) (*EmbeddingClient, error) {
	if projectID == "" {
		return nil, fmt.Errorf("projectID is required")
	}
	if location == "" {
		location = "us-central1"
	}
	if account == "" {
		account = "default"
	}

	tokenSource, err := auth.GetTokenSource(ctx, account, VertexAIScope)
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	return &EmbeddingClient{
		projectID:  projectID,
		location:   location,
		model:      ModelTextEmbedding005,
		httpClient: oauth2.NewClient(ctx, tokenSource),
		account:    account,
	}, nil
}

// WithModel sets the embedding model to use
func (c *EmbeddingClient) WithModel(model string) *EmbeddingClient {
	c.model = model
	return c
}

// embeddingRequest is the Vertex AI embedding API request format
type embeddingRequest struct {
	Instances  []embeddingInstance `json:"instances"`
	Parameters *embeddingParams    `json:"parameters,omitempty"`
}

type embeddingInstance struct {
	TaskType string `json:"task_type,omitempty"`
	Content  string `json:"content"`
}

type embeddingParams struct {
	OutputDimensionality int `json:"outputDimensionality,omitempty"`
}

// embeddingResponse is the Vertex AI embedding API response format
type embeddingResponse struct {
	Predictions []struct {
		Embeddings struct {
			Values     []float32 `json:"values"`
			Statistics struct {
				Truncated  bool `json:"truncated"`
				TokenCount int  `json:"token_count"`
			} `json:"statistics"`
		} `json:"embeddings"`
	} `json:"predictions"`
	Metadata struct {
		BillableCharacterCount int `json:"billableCharacterCount"`
	} `json:"metadata"`
}

// EmbeddingUsage tracks token usage for embedding operations
type EmbeddingUsage struct {
	TotalTokens            int
	BillableCharacterCount int
}

// TaskType defines the type of embedding task for better quality
type TaskType string

const (
	TaskTypeRetrievalDocument   TaskType = "RETRIEVAL_DOCUMENT"
	TaskTypeRetrievalQuery      TaskType = "RETRIEVAL_QUERY"
	TaskTypeSemainticSimilarity TaskType = "SEMANTIC_SIMILARITY"
	TaskTypeClassification      TaskType = "CLASSIFICATION"
	TaskTypeClustering          TaskType = "CLUSTERING"
)

// EmbedText generates an embedding for a single text
func (c *EmbeddingClient) EmbedText(ctx context.Context, text string, taskType TaskType) ([]float32, error) {
	embeddings, _, err := c.EmbedTexts(ctx, []string{text}, taskType)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedTextWithUsage generates an embedding for a single text and returns usage info
func (c *EmbeddingClient) EmbedTextWithUsage(ctx context.Context, text string, taskType TaskType) ([]float32, *EmbeddingUsage, error) {
	embeddings, usage, err := c.EmbedTexts(ctx, []string{text}, taskType)
	if err != nil {
		return nil, nil, err
	}
	if len(embeddings) == 0 {
		return nil, nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], usage, nil
}

// EmbedTexts generates embeddings for multiple texts (batch)
// Maximum 250 texts per batch for Vertex AI
// Returns embeddings and usage info (tokens consumed)
func (c *EmbeddingClient) EmbedTexts(ctx context.Context, texts []string, taskType TaskType) ([][]float32, *EmbeddingUsage, error) {
	if len(texts) == 0 {
		return nil, nil, nil
	}
	if len(texts) > 250 {
		return nil, nil, fmt.Errorf("maximum 250 texts per batch, got %d", len(texts))
	}

	// Build instances
	instances := make([]embeddingInstance, len(texts))
	for i, text := range texts {
		instances[i] = embeddingInstance{
			TaskType: string(taskType),
			Content:  truncateText(text, 10000), // Vertex AI has a ~10k char limit
		}
	}

	req := embeddingRequest{
		Instances: instances,
	}

	// Make API request
	endpoint := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		c.location, c.projectID, c.location, c.model,
	)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract embeddings and calculate usage
	embeddings := make([][]float32, len(embResp.Predictions))
	totalTokens := 0
	for i, pred := range embResp.Predictions {
		embeddings[i] = pred.Embeddings.Values
		totalTokens += pred.Embeddings.Statistics.TokenCount
	}

	usage := &EmbeddingUsage{
		TotalTokens:            totalTokens,
		BillableCharacterCount: embResp.Metadata.BillableCharacterCount,
	}

	return embeddings, usage, nil
}

// EmbedTextsBatched handles large text sets by splitting into batches
// Returns all embeddings and aggregated usage info
func (c *EmbeddingClient) EmbedTextsBatched(ctx context.Context, texts []string, taskType TaskType, batchSize int) ([][]float32, *EmbeddingUsage, error) {
	if batchSize <= 0 || batchSize > 250 {
		batchSize = 250
	}

	var allEmbeddings [][]float32
	totalUsage := &EmbeddingUsage{}

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, usage, err := c.EmbedTexts(ctx, batch, taskType)
		if err != nil {
			return nil, nil, fmt.Errorf("batch %d-%d failed: %w", i, end, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
		if usage != nil {
			totalUsage.TotalTokens += usage.TotalTokens
			totalUsage.BillableCharacterCount += usage.BillableCharacterCount
		}
	}

	return allEmbeddings, totalUsage, nil
}

// truncateText truncates text to the specified max length
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	// Try to truncate at a word boundary
	truncated := text[:maxLen]
	if idx := strings.LastIndex(truncated, " "); idx > maxLen-100 {
		return truncated[:idx]
	}
	return truncated
}

// PrepareTextForEmbedding prepares file content for embedding
// It combines filename, path, and content into a structured format
func PrepareTextForEmbedding(file *File) string {
	var sb strings.Builder

	// Add filename and path as context
	sb.WriteString(fmt.Sprintf("File: %s\n", file.Filename))
	sb.WriteString(fmt.Sprintf("Path: %s\n", file.Path))

	if file.Category != "" {
		sb.WriteString(fmt.Sprintf("Type: %s", file.Category))
		if file.Subcategory != "" {
			sb.WriteString(fmt.Sprintf("/%s", file.Subcategory))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Add content
	if file.ContentText != "" {
		sb.WriteString(file.ContentText)
	} else if file.ContentPreview != "" {
		sb.WriteString(file.ContentPreview)
	}

	return sb.String()
}
