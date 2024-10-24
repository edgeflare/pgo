package mqtt

import (
	"encoding/json"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/edgeflare/pgo/pkg/util/rand"
	"github.com/edgeflare/pgo/pkg/x/logrepl"
)

type PeerMQTT struct {
	client mqtt.Client
}

func (p *PeerMQTT) Publish(event logrepl.PostgresCDC) error {
	topic := fmt.Sprintf("/pgcdc/%s", event.Table)
	data, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	token := p.client.Publish(topic, 0, false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Printf("Publish error: %v", err)
		return err
	}
	return nil
}

func (p *PeerMQTT) Init(config json.RawMessage) error {
	type Config struct {
		Broker   string `json:"broker"`
		Username string `json:"username"`
		Password string `json:"password"`
		ClientID string `json:"client_id"`
	}

	var cfg Config
	if config != nil {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return fmt.Errorf("config parse error: %w", err)
		}
	}

	if cfg.Broker == "" {
		cfg.Broker = util.GetEnvOrDefault("PGO_MQTT_BROKER", "tcp://127.0.0.1:1883")
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)

	username := cfg.Username
	if username == "" {
		username = util.GetEnvOrDefault("PGO_MQTT_USERNAME", "")
	}
	opts.SetUsername(username)

	password := cfg.Password
	if password == "" {
		password = util.GetEnvOrDefault("PGO_MQTT_PASSWORD", "")
	}
	opts.SetPassword(password)

	clientID := cfg.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("pgo-logrepl-%s", rand.NewName())
	}
	opts.SetClientID(clientID)

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("broker connection error: %w", token.Error())
	}

	p.client = mqttClient
	log.Println("MQTT client initialized")

	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorMQTT, &PeerMQTT{})
}
