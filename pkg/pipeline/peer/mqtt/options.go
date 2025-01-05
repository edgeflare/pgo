package mqtt

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type ClientOptions struct {
	Servers                []*url.URL `json:"servers"`
	ClientID               string     `json:"clientID"`
	Username               string     `json:"username"`
	Password               string     `json:"password"`
	CredentialsProvider    mqtt.CredentialsProvider
	CleanSession           bool
	Order                  bool
	WillEnabled            bool
	WillTopic              string
	WillPayload            []byte
	WillQos                byte
	WillRetained           bool
	ProtocolVersion        uint
	TLSConfig              *tls.Config
	KeepAlive              int64 // Warning: Some brokers may reject connections with Keepalive = 0.
	PingTimeout            time.Duration
	ConnectTimeout         time.Duration
	MaxReconnectInterval   time.Duration
	AutoReconnect          bool
	ConnectRetryInterval   time.Duration
	ConnectRetry           bool
	Store                  mqtt.Store
	DefaultPublishHandler  mqtt.MessageHandler
	OnConnect              mqtt.OnConnectHandler
	OnConnectionLost       mqtt.ConnectionLostHandler
	OnReconnecting         mqtt.ReconnectHandler
	OnConnectAttempt       mqtt.ConnectionAttemptHandler
	WriteTimeout           time.Duration
	MessageChannelDepth    uint
	ResumeSubs             bool
	HTTPHeaders            http.Header
	WebsocketOptions       *mqtt.WebsocketOptions
	MaxResumePubInFlight   int // // 0 = no limit; otherwise this is the maximum simultaneous messages sent while resuming
	Dialer                 *net.Dialer
	CustomOpenConnectionFn mqtt.OpenConnectionFunc
	AutoAckDisabled        bool
}
