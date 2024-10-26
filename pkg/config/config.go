package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Postgres struct {
		LogReplConnString string `mapstructure:"logrepl_conn_string"`
		Tables            string `mapstructure:"tables"`
	} `mapstructure:"postgres"`
	Peers []PeerConfig `mapstructure:"peers"`
}

type PeerConfig struct {
	Name      string                 `mapstructure:"name"`
	Connector string                 `mapstructure:"connector"`
	Config    map[string]interface{} `mapstructure:"config"`
	Args      map[string]interface{} `mapstructure:"args"`
}

func LoadConfig(cfgFile string) (*Config, error) {
	v := viper.New()

	if cfgFile != "" {
		// Use config file from the flag if specified
		v.SetConfigFile(cfgFile)
	} else {
		// Search for config in default locations
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

func (c *Config) GetPeerConfig(peerName string) map[string]interface{} {
	for _, peer := range c.Peers {
		if peer.Name == peerName {
			return peer.Config
		}
	}
	return nil
}
