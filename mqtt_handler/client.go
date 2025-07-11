package mqtt_handler

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient encapsulates the MQTT client
type MQTTClient struct {
	client mqtt.Client
}

// NewClient creates and connects a new MQTT client
func NewClient(broker, clientID, user, password string) (*MQTTClient, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(user)
	opts.SetPassword(password)
	// Add more options as needed (e.g., automatic reconnection)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("error connecting to MQTT broker: %w", token.Error())
	}
	fmt.Println("Connected to MQTT broker")
	return &MQTTClient{client: client}, nil
}

// Subscribe subscribes to an MQTT topic
func (c *MQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	if token := c.client.Subscribe(topic, qos, callback); token.Wait() && token.Error() != nil {
		return fmt.Errorf("error subscribing to topic %s: %w", topic, token.Error())
	}
	fmt.Printf("Subscribed to topic: %s\n", topic)
	return nil
}

// Disconnect disconnects from the MQTT broker
func (c *MQTTClient) Disconnect() {
	c.client.Disconnect(250) // 250ms wait to finish
	fmt.Println("Disconnected from MQTT broker")
}
