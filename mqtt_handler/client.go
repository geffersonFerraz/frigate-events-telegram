package mqtt_handler

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient encapsula o cliente MQTT
type MQTTClient struct {
	client mqtt.Client
}

// NewClient cria e conecta um novo cliente MQTT
func NewClient(broker, clientID, user, password string) (*MQTTClient, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(user)
	opts.SetPassword(password)
	// Adicionar mais opções conforme necessário (ex: reconexão automática)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("erro ao conectar ao broker MQTT: %w", token.Error())
	}
	fmt.Println("Conectado ao broker MQTT")
	return &MQTTClient{client: client}, nil
}

// Subscribe inscreve em um tópico MQTT
func (c *MQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	if token := c.client.Subscribe(topic, qos, callback); token.Wait() && token.Error() != nil {
		return fmt.Errorf("erro ao inscrever no tópico %s: %w", topic, token.Error())
	}
	fmt.Printf("Inscrito no tópico: %s\n", topic)
	return nil
}

// Disconnect desconecta do broker MQTT
func (c *MQTTClient) Disconnect() {
	c.client.Disconnect(250) // 250ms de espera para finalizar
	fmt.Println("Desconectado do broker MQTT")
}
