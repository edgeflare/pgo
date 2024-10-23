// Package pipeline provides a framework for managing data pipelines
// from/to PostgreSQL to/from various `Peer`s (ie data source/destination).
//
// Supported peer types include ClickHouse, HTTP endpoints, Kafka,
// MQTT, and gRPC, with extensibility through Go plugins.
//
// It defines a `Connector` interface that all `Peer` types must implement.
package pipeline
