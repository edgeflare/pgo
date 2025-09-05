package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// TLSOptions holds TLS configuration that can be marshaled from JSON/YAML
type TLSOptions struct {
	InsecureSkipVerify bool   `json:"insecureSkipVerify" yaml:"insecureSkipVerify"`
	ServerName         string `json:"serverName,omitempty" yaml:"serverName,omitempty"`
	CAFile             string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	CertFile           string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile            string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	CACert             string `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	ClientCert         string `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey          string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
}

type ClientOptions struct {
	Store                  mqtt.Store
	OnConnectAttempt       mqtt.ConnectionAttemptHandler
	CustomOpenConnectionFn mqtt.OpenConnectionFunc
	DefaultPublishHandler  mqtt.MessageHandler
	CredentialsProvider    mqtt.CredentialsProvider
	HTTPHeaders            http.Header
	Dialer                 *net.Dialer
	WebsocketOptions       *mqtt.WebsocketOptions
	OnConnect              mqtt.OnConnectHandler
	OnConnectionLost       mqtt.ConnectionLostHandler
	OnReconnecting         mqtt.ReconnectHandler
	TLSConfig              *tls.Config
	TLS                    *TLSOptions `json:"tls,omitempty" yaml:"tls,omitempty"`
	ClientID               string      `json:"clientID"`
	WillTopic              string
	Username               string     `json:"username"`
	Password               string     `json:"password"`
	Servers                []*url.URL `json:"servers"`
	WillPayload            []byte
	WriteTimeout           time.Duration
	ConnectRetryInterval   time.Duration
	MaxResumePubInFlight   int
	MessageChannelDepth    uint
	MaxReconnectInterval   time.Duration
	ConnectTimeout         time.Duration
	PingTimeout            time.Duration
	KeepAlive              int64
	ProtocolVersion        uint
	WillRetained           bool
	AutoReconnect          bool
	ResumeSubs             bool
	WillQos                byte
	WillEnabled            bool
	ConnectRetry           bool
	Order                  bool
	CleanSession           bool
	AutoAckDisabled        bool
}

func createTLSConfig(tlsOpts *TLSOptions) (*tls.Config, error) {
	if tlsOpts == nil {
		return nil, nil
	}

	config := &tls.Config{
		InsecureSkipVerify: tlsOpts.InsecureSkipVerify,
		ServerName:         tlsOpts.ServerName,
	}

	// Load CA certificate
	if tlsOpts.CAFile != "" || tlsOpts.CACert != "" {
		caCertPool := x509.NewCertPool()

		var caCert []byte
		var err error

		if tlsOpts.CAFile != "" {
			caCert, err = os.ReadFile(tlsOpts.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file: %w", err)
			}
		} else {
			caCert = []byte(tlsOpts.CACert)
		}

		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		config.RootCAs = caCertPool
	}

	// Load client certificate and key
	if (tlsOpts.CertFile != "" && tlsOpts.KeyFile != "") ||
		(tlsOpts.ClientCert != "" && tlsOpts.ClientKey != "") {

		var cert tls.Certificate
		var err error

		if tlsOpts.CertFile != "" && tlsOpts.KeyFile != "" {
			cert, err = tls.LoadX509KeyPair(tlsOpts.CertFile, tlsOpts.KeyFile)
		} else {
			cert, err = tls.X509KeyPair([]byte(tlsOpts.ClientCert), []byte(tlsOpts.ClientKey))
		}

		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		config.Certificates = []tls.Certificate{cert}
	}

	return config, nil
}
