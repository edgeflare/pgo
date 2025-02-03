package mqtt

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/edgeflare/pgo/pkg/util/rand"
	"go.uber.org/zap"
)

type PeerMQTT struct {
	*Client
}

func (p *PeerMQTT) Pub(event pglogrepl.CDC, args ...any) error {
	// Create the topic using the trimmed prefix
	topic := fmt.Sprintf("%s/%s", p.publishTopicPrefix, event.Payload.Source.Table)
	data, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	return p.Client.Publish(topic, 0, false, data)
}

func (p *PeerMQTT) Connect(config json.RawMessage, args ...any) error {
	var opts ClientOptions

	// Unmarshal JSON into a temporary struct with servers as strings
	var tempOpts struct {
		ClientOptions
		Servers []string `json:"servers"`
	}

	if err := json.Unmarshal(config, &tempOpts); err != nil {
		return fmt.Errorf("failed to unmarshal MQTT config: %w", err)
	}

	// Copy the unmarshaled data to opts
	opts = tempOpts.ClientOptions

	// Convert string servers to url.URL
	for _, server := range tempOpts.Servers {
		u, err := url.Parse(server)
		if err != nil {
			return fmt.Errorf("failed to parse server URL %s: %w", server, err)
		}
		opts.Servers = append(opts.Servers, u) // Dereference the pointer
	}

	publishTopicPrefix := parseArgs(args)[0].(string)

	// Convert our ClientOptions to paho mqtt.ClientOptions
	mqttOpts := convertToPahoOptions(&opts)

	// Set default options
	setDefaultOptions(mqttOpts)

	// Create and embed MQTT client
	p.Client = NewClient(mqttOpts)

	// Connect to MQTT broker
	if err := p.Client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", err)
	}

	p.logger.Info("MQTT peer initialized",
		zap.Strings("brokers", getBrokerStrings(mqttOpts)),
		zap.String("client_id", mqttOpts.ClientID),
		zap.String("publish_topic_prefix", publishTopicPrefix))

	// Store trimmed publishTopicPrefix in the PeerMQTT struct
	p.publishTopicPrefix = strings.TrimRight(publishTopicPrefix, "/")

	return nil
}

func convertToPahoOptions(opts *ClientOptions) *mqtt.ClientOptions {
	pahoOpts := mqtt.NewClientOptions()

	// Convert Servers
	for _, server := range opts.Servers {
		pahoOpts.AddBroker(server.String())
	}

	// Set other options only if they are non-empty or non-nil
	if opts.ClientID != "" {
		pahoOpts.SetClientID(opts.ClientID)
	}
	if opts.Username != "" {
		pahoOpts.SetUsername(opts.Username)
	}
	if opts.Password != "" {
		pahoOpts.SetPassword(opts.Password)
	}
	if opts.TLSConfig != nil {
		pahoOpts.SetTLSConfig(opts.TLSConfig)
	}
	if opts.KeepAlive > 0 {
		pahoOpts.SetKeepAlive(time.Duration(opts.KeepAlive) * time.Second)
	}
	if opts.PingTimeout > 0 {
		pahoOpts.SetPingTimeout(opts.PingTimeout)
	}
	if opts.ConnectTimeout > 0 {
		pahoOpts.SetConnectTimeout(opts.ConnectTimeout)
	}
	if opts.MaxReconnectInterval > 0 {
		pahoOpts.SetMaxReconnectInterval(opts.MaxReconnectInterval)
	}
	if opts.ConnectRetryInterval > 0 {
		pahoOpts.SetConnectRetryInterval(opts.ConnectRetryInterval)
	}
	if opts.WriteTimeout > 0 {
		pahoOpts.SetWriteTimeout(opts.WriteTimeout)
	}
	if opts.MessageChannelDepth > 0 {
		pahoOpts.SetMessageChannelDepth(opts.MessageChannelDepth)
	}
	if opts.MaxResumePubInFlight > 0 {
		pahoOpts.SetMaxResumePubInFlight(opts.MaxResumePubInFlight)
	}

	// Set boolean options
	pahoOpts.SetCleanSession(opts.CleanSession)
	pahoOpts.SetOrderMatters(opts.Order)
	pahoOpts.SetAutoReconnect(opts.AutoReconnect)
	pahoOpts.SetConnectRetry(opts.ConnectRetry)
	pahoOpts.SetResumeSubs(opts.ResumeSubs)
	pahoOpts.SetAutoAckDisabled(opts.AutoAckDisabled)

	// Set non-primitive options only if non-nil
	if opts.Store != nil {
		pahoOpts.SetStore(opts.Store)
	}
	if opts.DefaultPublishHandler != nil {
		pahoOpts.SetDefaultPublishHandler(opts.DefaultPublishHandler)
	}
	if opts.OnConnect != nil {
		pahoOpts.SetOnConnectHandler(opts.OnConnect)
	}
	if opts.OnConnectionLost != nil {
		pahoOpts.SetConnectionLostHandler(opts.OnConnectionLost)
	}
	if opts.OnReconnecting != nil {
		pahoOpts.SetReconnectingHandler(opts.OnReconnecting)
	}
	if opts.OnConnectAttempt != nil {
		pahoOpts.SetConnectionAttemptHandler(opts.OnConnectAttempt)
	}
	if opts.HTTPHeaders != nil {
		pahoOpts.SetHTTPHeaders(opts.HTTPHeaders)
	}
	if opts.WebsocketOptions != nil {
		pahoOpts.SetWebsocketOptions(opts.WebsocketOptions)
	}
	if opts.Dialer != nil {
		pahoOpts.SetDialer(opts.Dialer)
	}
	if opts.CustomOpenConnectionFn != nil {
		pahoOpts.SetCustomOpenConnectionFn(opts.CustomOpenConnectionFn)
	}

	// Set will if enabled
	if opts.WillEnabled {
		pahoOpts.SetWill(opts.WillTopic, string(opts.WillPayload), opts.WillQos, opts.WillRetained)
	}

	return pahoOpts
}

func setDefaultOptions(opts *mqtt.ClientOptions) {
	if len(opts.Servers) == 0 {
		defaultBroker := util.GetEnvOrDefault("PGO_MQTT_BROKER", "tcp://127.0.0.1:1883")
		opts.AddBroker(defaultBroker)
	}

	if opts.Username == "" {
		opts.SetUsername(util.GetEnvOrDefault("PGO_MQTT_USERNAME", ""))
	}
	if opts.Password == "" {
		opts.SetPassword(util.GetEnvOrDefault("PGO_MQTT_PASSWORD", ""))
	}
	if opts.ClientID == "" {
		opts.SetClientID(fmt.Sprintf("pgo-logrepl-%s", rand.NewName()))
	}
}

func getBrokerStrings(opts *mqtt.ClientOptions) []string {
	brokers := make([]string, len(opts.Servers))
	for i, server := range opts.Servers {
		brokers[i] = server.String()
	}
	return brokers
}

// Add this function at the end of the file
func parseArgs(args []any) []any {
	var publishTopicPrefix string
	if len(args) > 0 && args[0] != nil {
		publishTopicPrefix = args[0].(string)
	}

	// Use default if empty or nil
	if publishTopicPrefix == "" {
		publishTopicPrefix = "/pgcdc"
	}

	// Ensure the prefix starts with "/"
	if !strings.HasPrefix(publishTopicPrefix, "/") {
		publishTopicPrefix = "/" + publishTopicPrefix
	}

	return []any{publishTopicPrefix}
}

func (p *PeerMQTT) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
	// TODO: Implement
	return nil, pipeline.ErrConnectorTypeMismatch
}

func (p *PeerMQTT) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePubSub
}

func (p *PeerMQTT) Disconnect() error {
	// TODO: Implement
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorMQTT, &PeerMQTT{})
}
