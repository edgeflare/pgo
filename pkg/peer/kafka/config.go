package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"
	"strings"

	"github.com/IBM/sarama"
)

var (
	kafkaConfig = KafkaConfig{
		Brokers:       os.Getenv("KAFKA_BROKERS"),
		Version:       sarama.DefaultVersion.String(), // "2.1.0",
		UserName:      os.Getenv("KAFKA_SASL_USERNAME"),
		Password:      os.Getenv("KAFKA_SASL_PASSWORD"),
		Algorithm:     "sha512",
		Topic:         os.Getenv("KAFKA_DEFAULT_TOPIC"),
		CertFile:      "",
		KeyFile:       "",
		CAFile:        "",
		TLSSkipVerify: false,
		UseTLS:        false,
		LogMsg:        true,
	}
)

var (
	logger = log.New(os.Stdout, "[Kafka] ", log.LstdFlags)
)

func init() {
	sarama.Logger = logger
}

type KafkaConfig struct {
	Brokers       string
	Version       string
	UserName      string
	Password      string
	Algorithm     string
	Topic         string
	CertFile      string
	KeyFile       string
	CAFile        string
	TLSSkipVerify bool
	UseTLS        bool
	LogMsg        bool
}

func createTLSConfiguration(tlsSkipVerify bool, certFile, keyFile, caFile string) (t *tls.Config) {
	t = &tls.Config{
		InsecureSkipVerify: tlsSkipVerify,
	}
	if certFile != "" && keyFile != "" && caFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := os.ReadFile(caFile)
		if err != nil {
			log.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: tlsSkipVerify,
		}
	}
	return t
}

func InitConfig(config KafkaConfig) (*sarama.Config, []string, error) {
	if config.Brokers == "" {
		logger.Fatalln("at least one broker is required")
	}
	splitBrokers := strings.Split(config.Brokers, ",")

	version, err := sarama.ParseKafkaVersion(config.Version)
	if err != nil {
		logger.Panicf("Error parsing Kafka version: %v", err)
	}

	if config.UserName == "" {
		logger.Fatalln("SASL username is required")
	}

	if config.Password == "" {
		logger.Fatalln("SASL password is required")
	}

	conf := sarama.NewConfig()
	conf.Producer.Retry.Max = 1
	conf.Producer.RequiredAcks = sarama.WaitForAll
	conf.Producer.Return.Successes = true
	conf.Version = version
	conf.ClientID = "sasl_scram_client"
	conf.Metadata.Full = true
	conf.Net.SASL.Enable = true
	conf.Net.SASL.User = config.UserName
	conf.Net.SASL.Password = config.Password
	conf.Net.SASL.Handshake = true
	if config.Algorithm == "sha512" {
		conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	} else if config.Algorithm == "sha256" {
		conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
	} else {
		log.Fatalf("invalid SHA algorithm \"%s\": can be either \"sha256\" or \"sha512\"", config.Algorithm)
	}

	if config.UseTLS {
		conf.Net.TLS.Enable = true
		conf.Net.TLS.Config = createTLSConfiguration(config.TLSSkipVerify, config.CertFile, config.KeyFile, config.CAFile)
	}

	return conf, splitBrokers, nil
}
