package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/edgeflare/pgo/pkg/httputil"
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
		Model:  c.Config.ModelId,
		Stream: false,
		Format: "json",
	}

	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", c.Config.ApiKey)},
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	body, err := httputil.Request(ctx, http.MethodPost, fmt.Sprintf("%s%s", c.Config.ApiUrl, c.Config.GeneratePath), dataBytes, headers, time.Minute*1)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	return body, nil
}

// GenerateWithRetrieval performs retrieval-augmented generation (RAG).
// It retrieves relevant information based on the given prompt or an optional
// retrieval query, then uses this information to augment the original prompt
// before generating a response.
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation.
//   - prompt: The main prompt or question to be answered by the language model.
//   - retrievalLimit: The maximum number of relevant documents to retrieve.
//   - retrievalInput: An optional query used specifically for retrieving relevant
//     documents. If not provided or empty, the prompt will be used for retrieval.
//
// Returns:
//   - []byte: The generated response from the language model.
//   - error: An error if any step in the process fails.
//
// The function follows these steps:
//  1. Retrieve relevant information using either the retrievalInput (if provided) or the prompt.
//  2. Construct an augmented prompt that includes the retrieved information and the original prompt.
//  3. Generate a response using the augmented prompt.
//
// This method allows for more flexible and potentially more accurate responses
// by incorporating relevant context into the generation process.
func (c *Client) GenerateWithRetrieval(ctx context.Context, prompt string, retrievalLimit int, retrievalInput ...string) ([]byte, error) {
	// Determine the query to use for retrieval
	query := prompt
	if len(retrievalInput) > 0 && retrievalInput[0] != "" {
		query = retrievalInput[0]
	}

	// Step 1: Retrieve relevant information
	relevantInfo, err := c.Retrieve(ctx, query, retrievalLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve relevant information: %w", err)
	}

	// Step 2: Construct an augmented prompt
	augmentedPrompt := constructAugmentedPrompt(prompt, relevantInfo)

	// Step 3: Generate response using the augmented prompt
	response, err := c.Generate(ctx, augmentedPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	return response, nil
}

// constructAugmentedPrompt creates a prompt that includes the original query and relevant information
func constructAugmentedPrompt(prompt string, relevantInfo []Embedding) string {
	var sb strings.Builder
	sb.WriteString("Given the following context:\n\n")

	for _, info := range relevantInfo {
		sb.WriteString(fmt.Sprintf("- %s\n", info.Content))
	}

	sb.WriteString(fmt.Sprintf("\nAnswer the following question: %s", prompt))

	return sb.String()
}
