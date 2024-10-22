# pgo (`/pɪɡəʊ/`): Postgres integrations in Go

> This is a piecemeal collection of code snippets, which has evolved from my Postgres+Go projects whenever they were reused, without any plan for a library.

Now I'm (forcibly) moving from applications the boilerplate / heavy-lifting around:

- net/http.Handler
  - router
  - middleware (logging, authentication, CORS, RequestID, ...)
  - Postgres middleware for attaching pgxpool.Conn for authorized user; useful for RLS
- Retrieval Augmented Generation (RAG)
  - embeddings from LLM APIs (OpenAI, Groq, Anthropic, Google, Ollama, ...)
  - pgvector search, augmented generation
- Pipelines (realtime/batch)
  - Postgres' Logical Replication (optionally LISTEN/NOTIFY)
  - MQTT, Kafka, HTTP, ClickHouse, gRPC (add more by writing plugins yourself)

## Usage

This reusability seems useful, maybe because I'm also learning by being forced to write reliable code that other code might depend on. Most of it actually isn’t dependable, yet. If you're curious, start browsing the [examples](./examples/), skim any doc.go, *.md files.

## Contributing
Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License
Apache License 2.0
