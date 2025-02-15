# pgo (`/pɪɡəʊ/`): Postgres integrations in Go

> This is a piecemeal collection of code snippets, which has evolved from my Postgres+Go projects whenever they were reused, without any plan/structure.

Now I'm (forcibly) moving from applications to this repo the boilerplate / heavy-lifting around:

- net/http.Handler
  - router
  - middleware (authentication, logging, CORS, RequestID, ...)
  - Postgres middleware attaches a pgxpool.Conn to request context for authorized user; useful for RLS
- Retrieval Augmented Generation (RAG)
  - fetch embeddings from LLM APIs (OpenAI, Groq, Anthropic, Google, Ollama, ...)
  - utils for pgvector search, augmented generation
- Pipelines (realtime/batch)
  - Postgres' Logical Replication
  - NATS, MQTT, Kafka, HTTP, ClickHouse, gRPC (add more by writing plugins yourself)

  <img src="https://raw.githubusercontent.com/edgeflare/pgo/refs/heads/main/docs/img/pgo.png" alt="pgo - postgres integrations in go" style="height: 36rem;">

## Usage

- #### As a standalone binary for publishing Postgres CDC to Kafka, MQTT, ClickHouse, etc, see [docs/postgres-cdc.md](./docs/postgres-cdc.md)

It's also possible to import functions, etc. This reusability seems helpful, mostly because I'm learning by being forced to write reliable code that other code might depend on. Most of it actually isn’t dependable, yet. If you're curious, start by browsing the [examples](./examples/), skimming over any doc.go, *.md files.

## Contributing
Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License
Apache License 2.0
