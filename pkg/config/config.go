package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/spf13/viper"
)

type Config struct {
	pipeline.Config
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
	var pipelineConfig pipeline.Config

	if err := v.Unmarshal(&pipelineConfig); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}
	cfg.Config = pipelineConfig

	return &cfg, nil
}
