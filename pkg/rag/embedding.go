package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/edgeflare/pgo/pkg/util/httpclient"
)

// EmbeddingRequest is the request body for the FetchEmbedding function
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse is the response body for the FetchEmbedding function
// https://platform.openai.com/docs/api-reference/embeddings/create
// https://github.com/ollama/ollama/blob/main/docs/api.md#embeddings
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// FetchEmbedding fetches embeddings from the LLM API
func (c *Client) FetchEmbedding(ctx context.Context, input []string) ([][]float32, error) {
	// check if input is empty
	if len(input) == 0 {
		return [][]float32{}, fmt.Errorf("input is empty")
	}

	data := &EmbeddingRequest{
		Input: input,
		Model: c.Config.ModelId,
	}

	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", c.Config.ApiKey)},
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return [][]float32{}, fmt.Errorf("failed to marshal request data: %w", err)
	}

	body, err := httpclient.Request(ctx, http.MethodPost, fmt.Sprintf("%s%s", c.Config.ApiUrl, c.Config.EmbeddingsPath), dataBytes, headers)
	if err != nil {
		return [][]float32{}, fmt.Errorf("failed to fetch embeddings: %w", err)
	}

	var response EmbeddingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return [][]float32{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	embeddings := make([][]float32, len(response.Data))
	for i, d := range response.Data {
		embeddings[i] = d.Embedding
	}

	return embeddings, nil
}
