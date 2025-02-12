package pipeline

import "github.com/edgeflare/pgo/pkg/pipeline/transform"

// Source is a pipeline input with its transformations.
type Source struct {
	// Name must match one of configured peers
	Name string `mapstructure:"name"`
	// Source transformations are applied (in the order specified) as soon as CDC event is received before any processing.
	Transformations []transform.Transformation `mapstructure:"transformations"`
}

// Sink is a pipeline output with its transformations.
type Sink struct {
	// Name must match one of configured peers
	Name string `mapstructure:"name"`
	// Sink-specific transformations are applied after source transformations, pipeline transformations and before sending to speceific sink
	Transformations []transform.Transformation `mapstructure:"transformations"`
}

// Pipeline configures a complete data processing pipeline.
type Pipeline struct {
	Name    string   `mapstructure:"name"`
	Sources []Source `mapstructure:"sources"`
	// Pipeline transformations are applied after source transformations and before sink transformations.
	// These are applied to all CDC events flowing through a pipeline from its all sources to all sinks
	Transformations []transform.Transformation `mapstructure:"transformations"`
	Sinks           []Sink                     `mapstructure:"sinks"`
}
