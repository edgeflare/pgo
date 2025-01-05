package config

import (
	"fmt"

	"github.com/edgeflare/pgo/pkg/pipeline/transform"
	"github.com/spf13/viper"
)

type Config struct {
	Peers     []Peer           `mapstructure:"peers"`
	Pipelines []PipelineConfig `mapstructure:"pipelines"`
}

type Peer struct {
	Name      string                 `mapstructure:"name"`
	Connector string                 `mapstructure:"connector"`
	Config    map[string]interface{} `mapstructure:"config"`
}

type PipelineConfig struct {
	Name            string                      `mapstructure:"name"`
	Sources         []SourceConfig              `mapstructure:"sources"`
	Sinks           []SinkConfig                `mapstructure:"sinks"`
	Transformations []transform.TransformConfig `mapstructure:"transformations"`
}

type SourceConfig struct {
	Name            string                      `mapstructure:"name"`
	Transformations []transform.TransformConfig `mapstructure:"transformations"`
}

type SinkConfig struct {
	Name            string                      `mapstructure:"name"`
	Transformations []transform.TransformConfig `mapstructure:"transformations"`
}

// TransformConfig represents a single transformation step
// type TransformConfig struct {
// 	transform.TransformConfig
// 	// Type   string                 `mapstructure:"type"`
// 	// Config map[string]interface{} `mapstructure:"config"`
// }

// Rest of the existing config code...

// package config

// import (
// 	"fmt"

// 	"github.com/spf13/viper"
// )

// type Config struct {
// 	Peers     []Peer           `mapstructure:"peers"`
// 	Pipelines []PipelineConfig `mapstructure:"pipelines"`
// }

// type Peer struct {
// 	Name      string                 `mapstructure:"name"`
// 	Connector string                 `mapstructure:"connector"`
// 	Config    map[string]interface{} `mapstructure:"config"`
// }

// type PipelineConfig struct {
// 	Name            string               `mapstructure:"name"`
// 	Sources         []SourceConfig       `mapstructure:"sources"`
// 	Sinks           []SinkConfig         `mapstructure:"sinks"`
// 	Transformations TransformationConfig `mapstructure:"transformations"`
// }

// type SourceConfig struct {
// 	Name            string               `mapstructure:"name"`
// 	Transformations TransformationConfig `mapstructure:"transformations"`
// }

// type SinkConfig struct {
// 	Name            string               `mapstructure:"name"`
// 	Transformations TransformationConfig `mapstructure:"transformations"`
// }

// type TransformationConfig struct {
// 	Extract  *ExtractTransform `mapstructure:"extract,omitempty"`
// 	Replace  *ReplaceTransform `mapstructure:"replace,omitempty"`
// 	Template *string           `mapstructure:"template,omitempty"`
// }

// type ExtractTransform struct {
// 	Fields []string `mapstructure:"fields"`
// }

// type ReplaceTransform struct {
// 	Table  map[string]string `mapstructure:"table"`
// 	Fields map[string]string `mapstructure:"fields"`
// }

func LoadConfig(cfgFile string) (*Config, error) {
	v := viper.New()
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("pgo")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.config")
		v.AddConfigPath(".")
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("PGO")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		fmt.Println("Using config file:", v.ConfigFileUsed())
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	return &cfg, nil
}

// Helper functions to look up configurations
func (c *Config) GetPeer(peerName string) *Peer {
	for _, peer := range c.Peers {
		if peer.Name == peerName {
			return &peer
		}
	}
	return nil
}

func (c *Config) GetPipeline(pipelineName string) *PipelineConfig {
	for _, pipeline := range c.Pipelines {
		if pipeline.Name == pipelineName {
			return &pipeline
		}
	}
	return nil
}

// GetSourcePeers returns all peer configs that are configured as sources in any pipeline
func (c *Config) GetSourcePeers() []Peer {
	sourceMap := make(map[string]Peer)
	for _, pipeline := range c.Pipelines {
		for _, source := range pipeline.Sources {
			if peer := c.GetPeer(source.Name); peer != nil {
				sourceMap[peer.Name] = *peer
			}
		}
	}

	sources := make([]Peer, 0, len(sourceMap))
	for _, peer := range sourceMap {
		sources = append(sources, peer)
	}
	return sources
}
