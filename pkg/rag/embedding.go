package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/edgeflare/pgo/pkg/util/httpclient"
)

// EmbeddingRequest is passed as body in FetchEmbeddings
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingsResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string          `json:"model"`
	Usage json.RawMessage `json:"usage"`
}

// Embeddings fetches embeddings from the API
func (c *Client) FetchEmbeddings(ctx context.Context, input []string) (EmbeddingsResponse, error) {
	data := &EmbeddingRequest{
		Input: input,
		Model: c.config.ModelId,
	}

	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", c.config.ApiKey)},
	}

	body, err := httpclient.Request(ctx, http.MethodPost, fmt.Sprintf("%s/v1/embeddings", c.config.ApiUrl), data, headers)
	if err != nil {
		return EmbeddingsResponse{}, fmt.Errorf("failed to fetch embeddings: %w", err)
	}

	var response EmbeddingsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return EmbeddingsResponse{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}
