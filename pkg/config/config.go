package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/spf13/viper"
)

type Config struct {
	Peers     []pipeline.Peer     `mapstructure:"peers"`
	Pipelines []pipeline.Pipeline `mapstructure:"pipelines"`
}

func Load(cfgFile string) (*Config, error) {
	v := viper.New()
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("pgo")
		v.SetConfigType("yaml")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config"))
		}
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

func (c *Config) GetPeer(peerName string) *pipeline.Peer {
	for _, peer := range c.Peers {
		if peer.Name == peerName {
			return &peer
		}
	}
	return nil
}

func (c *Config) GetPipeline(pipelineName string) *pipeline.Pipeline {
	for _, pipeline := range c.Pipelines {
		if pipeline.Name == pipelineName {
			return &pipeline
		}
	}
	return nil
}
