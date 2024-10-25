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
	var opts *mqtt.ClientOptions

	if config != nil {
		if err := json.Unmarshal(config, &opts); err != nil {
			return fmt.Errorf("config parse error: %w", err)
		}
	} else {
		opts = mqtt.NewClientOptions()
	}

	// Set default broker if not provided
	if len(opts.Servers) == 0 {
		opts.AddBroker(util.GetEnvOrDefault("PGO_MQTT_BROKER", "tcp://127.0.0.1:1883"))
	}

	// Set default username if not provided
	if opts.Username == "" {
		opts.SetUsername(util.GetEnvOrDefault("PGO_MQTT_USERNAME", ""))
	}

	// Set default password if not provided
	if opts.Password == "" {
		opts.SetPassword(util.GetEnvOrDefault("PGO_MQTT_PASSWORD", ""))
	}

	// Set default client ID if not provided
	if opts.ClientID == "" {
		opts.SetClientID(fmt.Sprintf("pgo-logrepl-%s", rand.NewName()))
	}

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
