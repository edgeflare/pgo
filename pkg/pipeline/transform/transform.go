package transform

import (
	"fmt"
	"sync"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
)

// TransformFunc is the signature for all transformation functions
type TransformFunc func(*pglogrepl.CDC) (*pglogrepl.CDC, error)

// Config is the interface that all transformation configs must implement
type Config interface {
	// Validate validates the configuration
	Validate() error
	// Type returns the transformation type
	Type() string
}

// Registry is a collection of transformation functions
type Registry struct {
	transforms sync.Map // map[string]func(Config) TransformFunc
}

// Register adds a transformation to the registry
func (r *Registry) Register(name string, factory func(Config) TransformFunc) {
	r.transforms.Store(name, factory)
}

// Get returns a transformation from the registry
func (r *Registry) Get(name string) (func(Config) TransformFunc, error) {
	if value, ok := r.transforms.Load(name); ok {
		return value.(func(Config) TransformFunc), nil
	}
	return nil, fmt.Errorf("transformation %s not found", name)
}

// NewRegistry creates a new transformation registry
func NewRegistry() *Registry {
	return &Registry{
		transforms: sync.Map{},
	}
}
