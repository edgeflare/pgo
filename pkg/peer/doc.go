// Package peer provides a flexible framework for connecting and publishing data
// to various external systems and services.
//
// The package defines a Connector interface that must be implemented by all
// peer types. This interface allows for a unified approach to initializing
// connections and publishing data across different backends.
//
// Supported peer types include:
//   - ClickHouse
//   - Debug (for testing and development)
//   - HTTP endpoints
//   - Kafka
//   - MQTT
//   - gRPC
//
// The package also supports dynamic loading of additional peer types through
// Go plugins, allowing for extensibility without modifying the core package.
//
// Primary use cases include broadcasting and publishing PostgreSQL Change Data
// Capture (CDC) events, which are captured via logical replication, to various
// supported peer types.
//
// This package is designed to be thread-safe and efficient for high-throughput
// data publishing scenarios.
package peer
