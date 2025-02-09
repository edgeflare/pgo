package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
	pb "github.com/edgeflare/pgo/proto/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PeerGRPC implements both source and sink functionality for gRPC
type PeerGRPC struct {
	pipeline.Peer
	server *grpc.Server
	client pb.CDCStreamClient
	events chan pglogrepl.CDC
	conn   *grpc.ClientConn
	mu     sync.RWMutex
}

// streamServer implements the gRPC server for CDC streaming
type streamServer struct {
	pb.UnimplementedCDCStreamServer
	events chan pglogrepl.CDC
}

func (s *streamServer) Stream(_ *pb.StreamRequest, stream pb.CDCStream_StreamServer) error {
	for event := range s.events {
		if event.Payload.After == nil {
			continue
		}

		data, err := json.Marshal(event.Payload.After)
		if err != nil {
			continue
		}

		if err := stream.Send(&pb.CDCEvent{
			Table: event.Payload.Source.Schema + "." + event.Payload.Source.Table,
			Data:  data,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Connect initializes the gRPC peer based on configuration
func (p *PeerGRPC) Connect(config json.RawMessage, args ...any) error {
	var cfg struct {
		Address  string `json:"address"`  // e.g., "localhost:50051"
		IsServer bool   `json:"isServer"` // true for server mode, false for client mode
		TLS      struct {
			Enabled  bool   `json:"enabled"`
			CertFile string `json:"certFile"`
			KeyFile  string `json:"keyFile"`
			CAFile   string `json:"caFile"`
		} `json:"tls"`
	}

	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("failed to parse gRPC config: %w", err)
	}

	if cfg.Address == "" {
		return fmt.Errorf("gRPC address is required")
	}

	p.events = make(chan pglogrepl.CDC, 100)

	if cfg.IsServer {
		return p.startServer(cfg)
	}
	return p.connectClient(cfg)
}

func (p *PeerGRPC) startServer(cfg struct {
	Address  string `json:"address"`
	IsServer bool   `json:"isServer"`
	TLS      struct {
		Enabled  bool   `json:"enabled"`
		CertFile string `json:"certFile"`
		KeyFile  string `json:"keyFile"`
		CAFile   string `json:"caFile"`
	} `json:"tls"`
}) error {
	lis, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	var opts []grpc.ServerOption
	if cfg.TLS.Enabled {
		// TODO
		fmt.Println("TO BE IMPLEMENTED")
	}

	p.server = grpc.NewServer(opts...)
	pb.RegisterCDCStreamServer(p.server, &streamServer{events: p.events})

	go func() {
		if err := p.server.Serve(lis); err != nil {
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	return nil
}

func (p *PeerGRPC) connectClient(cfg struct {
	Address  string `json:"address"`
	IsServer bool   `json:"isServer"`
	TLS      struct {
		Enabled  bool   `json:"enabled"`
		CertFile string `json:"certFile"`
		KeyFile  string `json:"keyFile"`
		CAFile   string `json:"caFile"`
	} `json:"tls"`
}) error {
	var opts []grpc.DialOption

	if !cfg.TLS.Enabled {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// TODO
		fmt.Println("TO BE IMPLEMENTED")
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	p.conn = conn
	p.client = pb.NewCDCStreamClient(conn)
	return nil
}

// Pub implements the sink functionality
func (p *PeerGRPC) Pub(event pglogrepl.CDC, args ...any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.events != nil {
		p.events <- event
	}
	return nil
}

// Sub implements the source functionality
func (p *PeerGRPC) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected to gRPC server")
	}

	stream, err := p.client.Stream(context.Background(), &pb.StreamRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	events := make(chan pglogrepl.CDC, 100)
	go func() {
		defer close(events)
		for {
			event, err := stream.Recv()
			if err != nil {
				return
			}

			var payload struct {
				Before interface{} `json:"before"`
				After  interface{} `json:"after"`
				Source struct {
					Schema string `json:"schema"`
					Table  string `json:"table"`
				} `json:"source"`
			}

			if err := json.Unmarshal(event.Data, &payload); err != nil {
				continue
			}

			events <- pglogrepl.CDC{
				Payload: struct {
					Before interface{} `json:"before"`
					After  interface{} `json:"after"`
					Source struct {
						Version   string `json:"version"`
						Connector string `json:"connector"`
						Name      string `json:"name"`
						TsMs      int64  `json:"ts_ms"`
						Snapshot  bool   `json:"snapshot"`
						Db        string `json:"db"`
						Sequence  string `json:"sequence"`
						Schema    string `json:"schema"`
						Table     string `json:"table"`
						TxID      int64  `json:"txId"`
						Lsn       int64  `json:"lsn"`
						Xmin      *int64 `json:"xmin,omitempty"`
					} `json:"source"`
					Op          string `json:"op"`
					TsMs        int64  `json:"ts_ms"`
					Transaction *struct {
						ID                  string `json:"id"`
						TotalOrder          int64  `json:"total_order"`
						DataCollectionOrder int64  `json:"data_collection_order"`
					} `json:"transaction,omitempty"`
				}{
					Before: payload.Before,
					After:  payload.After,
					Source: struct {
						Version   string `json:"version"`
						Connector string `json:"connector"`
						Name      string `json:"name"`
						TsMs      int64  `json:"ts_ms"`
						Snapshot  bool   `json:"snapshot"`
						Db        string `json:"db"`
						Sequence  string `json:"sequence"`
						Schema    string `json:"schema"`
						Table     string `json:"table"`
						TxID      int64  `json:"txId"`
						Lsn       int64  `json:"lsn"`
						Xmin      *int64 `json:"xmin,omitempty"`
					}{
						Schema: payload.Source.Schema,
						Table:  payload.Source.Table,
					},
				},
			}
		}
	}()

	return events, nil
}

// Type returns the connector type (PubSub since it supports both)
func (p *PeerGRPC) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePubSub
}

// Disconnect cleans up resources
func (p *PeerGRPC) Disconnect() error {
	if p.server != nil {
		p.server.GracefulStop()
	}
	if p.conn != nil {
		p.conn.Close()
	}
	close(p.events)
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorGRPC, &PeerGRPC{})
}
