// adopted from https://github.com/pgvector/pgvector-go/blob/master/examples/openai/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/edgeflare/pgo/pkg/util"
	"github.com/edgeflare/pgo/pkg/util/httpclient"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

var (
	apiUrl     = "http://127.0.0.1:11434/v1/embeddings" // ollama
	apikey     = util.GetEnvOrDefault("API_KEY", "")
	modelId    = "llama3.2:latest"
	dimensions = 3072 // for llama3.2. 1536 for openai
)

func main() {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, "postgres://postgres:secret@localhost:5432/postgres")
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		panic(err)
	}

	err = pgxvector.RegisterTypes(ctx, conn)
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS documents")
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE TABLE documents (id bigserial PRIMARY KEY, content text, embedding vector(%v))", dimensions))
	if err != nil {
		panic(err)
	}

	input := []string{
		"The dog is barking",
		"The cat is purring",
		"The bear is growling",
	}
	embeddings, err := FetchEmbeddings(input, apikey)
	if err != nil {
		panic(err)
	}

	for i, content := range input {
		_, err := conn.Exec(ctx, "INSERT INTO documents (content, embedding) VALUES ($1, $2)", content, pgvector.NewVector(embeddings[i]))
		if err != nil {
			panic(err)
		}
	}

	documentId := 1
	rows, err := conn.Query(ctx, "SELECT id, content FROM documents WHERE id != $1 ORDER BY embedding <=> (SELECT embedding FROM documents WHERE id = $1) LIMIT 5", documentId)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var content string
		err = rows.Scan(&id, &content)
		if err != nil {
			panic(err)
		}
		fmt.Println(id, content)
	}

	if rows.Err() != nil {
		panic(rows.Err())
	}
}

type apiRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type apiResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func FetchEmbeddings(input []string, apiKey string) ([][]float32, error) {
	data := &apiRequest{
		Input: input,
		Model: modelId,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", apiKey)},
	}

	body, err := httpclient.Request(ctx, http.MethodPost, apiUrl, data, headers)
	if err != nil {
		log.Fatal(err)
	}

	var response apiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	var embeddings [][]float32
	for _, item := range response.Data {
		embeddings = append(embeddings, item.Embedding)
	}
	return embeddings, nil
}
