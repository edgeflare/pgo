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
	ClientID               string `json:"clientID"`
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
