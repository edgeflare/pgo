package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/IBM/sarama"
)

// Config represents the Kafka configuration options
type Config struct {
	Brokers       []string
	Version       string
	SASL          SASLConfig
	TLS           TLSConfig
	ProducerTopic string
}

// SASLConfig represents SASL authentication configuration
type SASLConfig struct {
	Enable    bool
	Username  string
	Password  string
	Algorithm string // "sha256" or "sha512"
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enable     bool
	CertFile   string
	KeyFile    string
	CAFile     string
	SkipVerify bool
}

// NewConfig creates a new Kafka configuration with default values
func NewConfig() *Config {
	return &Config{
		Version: sarama.DefaultVersion.String(),
		SASL: SASLConfig{
			Algorithm: "sha512",
		},
	}
}

// ToSaramaConfig converts the Config to a sarama.Config
func (c *Config) ToSaramaConfig() (*sarama.Config, error) {
	conf := sarama.NewConfig()

	// Set Kafka version
	version, err := sarama.ParseKafkaVersion(c.Version)
	if err != nil {
		return nil, fmt.Errorf("error parsing Kafka version: %w", err)
	}
	conf.Version = version

	// Configure SASL
	if c.SASL.Enable {
		conf.Net.SASL.Enable = true
		conf.Net.SASL.User = c.SASL.Username
		conf.Net.SASL.Password = c.SASL.Password
		conf.Net.SASL.Handshake = true

		switch c.SASL.Algorithm {
		case "sha512":
			conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
			conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		case "sha256":
			conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
			conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		default:
			return nil, fmt.Errorf("invalid SASL algorithm: %s", c.SASL.Algorithm)
		}
	}

	// Configure TLS
	if c.TLS.Enable {
		conf.Net.TLS.Enable = true
		conf.Net.TLS.Config = createTLSConfiguration(c.TLS)
	}

	// Set other default configurations
	conf.Producer.Retry.Max = 1
	conf.Producer.RequiredAcks = sarama.WaitForAll
	conf.Producer.Return.Successes = true
	conf.ClientID = "sasl_scram_client"
	conf.Metadata.Full = true

	return conf, nil
}

func createTLSConfiguration(tlsConfig TLSConfig) *tls.Config {
	t := &tls.Config{
		InsecureSkipVerify: tlsConfig.SkipVerify,
	}

	if tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" && tlsConfig.CAFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil
		}

		caCert, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t.Certificates = []tls.Certificate{cert}
		t.RootCAs = caCertPool
	}

	return t
}

// GetBrokers returns the list of Kafka brokers
func (c *Config) GetBrokers() []string {
	return c.Brokers
}