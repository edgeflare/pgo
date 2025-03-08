package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/spf13/viper"
)

// Config holds application-wide configuration
type Config struct {
	REST     RESTConfig      `mapstructure:"rest"`
	Pipeline pipeline.Config `mapstructure:"pipeline"`
}

type RESTConfig struct {
	PG              PGConfig   `mapstructure:"pg"`
	ListenAddr      string     `mapstructure:"listenAddr"`
	BaseURL         string     `mapstructure:"baseURL"`
	OIDC            OIDCConfig `mapstructure:"oidc"`
	BasicAuth       bool       `mapstructure:"basicAuthEnabled"` // TODO: add support for basic-auth
	AnonRoleEnabled bool       `mapstructure:"anonRoleEnabled"`  // TODO: add support for anon auth
}

type PGConfig struct {
	ConnString string `mapstructure:"connString"`
}

type OIDCConfig struct {
	ClientID     string `mapstructure:"clientID"`
	ClientSecret string `mapstructure:"clientSecret"`
	Issuer       string `mapstructure:"issuer"`
	RoleClaimKey string `mapstructure:"roleClaimKey"`
}

func DefaultRESTConfig() RESTConfig {
	return RESTConfig{
		ListenAddr:      ":8080",
		AnonRoleEnabled: true,
		OIDC: OIDCConfig{
			RoleClaimKey: ".policies.pgrole",
		},
	}
}

// Load reads config from file or environment
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
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	return &cfg, nil
}
