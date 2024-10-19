package rag

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/edgeflare/pgo/pkg/util/httpclient"
)

// GenerateRequest is the body for /generate requests. Model and Prompt fields are required.
type GenerateRequest struct {
	// Model is the model name; it should be a name familiar to Ollama from
	// the library at https://ollama.com/library
	Model string `json:"model"`

	// Prompt is the textual prompt to send to the model.
	Prompt string `json:"prompt"`

	// Suffix is the text that comes after the inserted text.
	Suffix string `json:"suffix"`

	// System overrides the model's default system message/prompt.
	System string `json:"system"`

	// Template overrides the model's default prompt template.
	Template string `json:"template"`

	// Context is the context parameter returned from a previous call to
	// Generate call. It can be used to keep a short conversational memory.
	Context []int `json:"context,omitempty"`

	// Stream specifies whether the response is streaming; it is true by default.
	Stream bool `json:"stream"` // ollama uses *bool

	// Raw set to true means that no formatting will be applied to the prompt.
	Raw bool `json:"raw,omitempty"`

	// Format specifies the format to return a response in.
	Format string `json:"format"`

	// KeepAlive controls how long the model will stay loaded in memory following
	// this request.
	KeepAlive *time.Duration `json:"keep_alive,omitempty"`

	// Images is an optional list of base64-encoded images accompanying this
	// request, for multimodal models.
	Images []string `json:"images,omitempty"`

	// Options lists model-specific options. For example, temperature can be
	// set through this field, if the model supports it.
	Options map[string]interface{} `json:"options"`
}

// type GenerateResponse struct {
// 	Response string          `json:"response"`
// 	Model    string          `json:"model"`
// 	Usage    json.RawMessage `json:"usage,omitempty"`
// }

// Generate sends a generation request to the API and returns the response
func (c *Client) Generate(ctx context.Context, prompt string) ([]byte, error) {
	data := GenerateRequest{
		Prompt: prompt,
		Model:  c.config.ModelId,
		Stream: false,
		Format: "json",
	}

	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", c.config.ApiKey)},
	}

	body, err := httpclient.Request(ctx, http.MethodPost, fmt.Sprintf("%s/api/generate", c.config.ApiUrl), data, headers, time.Minute*1)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	return body, nil
}

// GenerateWithRetrieval sends a generation request to the API after retrieving relevant information
// based on the provided prompt.
func (c *Client) GenerateWithRetrieval(ctx context.Context, prompt string, limit int) ([]byte, error) {
	// Retrieve embeddings for user promt
	retrievedEmbeddings, err := c.Retrieve(ctx, prompt, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve embeddings: %w", err)
	}

	// Construct the new prompt
	var contextBuilder string
	for _, embedding := range retrievedEmbeddings {
		contextBuilder += fmt.Sprintf("Content: %s\n", embedding.Content)
	}

	// Combine the context with the original prompt
	newPrompt := fmt.Sprintf("Here are some relevant pieces of information:\n%s\n\nUsing this context, %s", contextBuilder, prompt)

	// Send the generation request
	response, err := c.Generate(ctx, newPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	return response, nil
}
