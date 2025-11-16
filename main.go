package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/geffersonFerraz/frigate-events/config"
	"github.com/geffersonFerraz/frigate-events/frigate"
	"github.com/geffersonFerraz/frigate-events/frigate/events"
	mqtt_handler "github.com/geffersonFerraz/frigate-events/mqtt"
	rabbit_handler "github.com/geffersonFerraz/frigate-events/rabbit"
	"github.com/geffersonFerraz/frigate-events/redis_handler"
	telegram_handler "github.com/geffersonFerraz/frigate-events/telegram"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		log.Println("use ./frigate-events [telegram|mqtt]")
		os.Exit(1)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	redis, err := redis_handler.NewRedisHandler(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Error initializing Redis: %v", err)
	}
	defer redis.Close()

	frigate := frigate.NewFrigate(cfg.FrigateURL)

	rabbit, err := rabbit_handler.NewRabbit(cfg)
	if err != nil {
		log.Fatalf("Error initializing RabbitMQ: %v", err)
	}
	defer rabbit.Close()

	rabbit.DeclareQueue(cfg.RabbitMQQueue, cfg.RabbitMQDurable)
	rabbit.DeclareExchange(cfg.RabbitMQExchange, cfg.RabbitMQDurable)
	rabbit.DeclareBinding(cfg.RabbitMQQueue, cfg.RabbitMQExchange, cfg.RabbitMQRoutingKey)

	ctx := context.Background()

	switch args[0] {
	case "telegram": // process rabbit message and send to telegram
		tgBot, err := telegram_handler.NewBot(telegram_handler.TelegramBot{
			Token:         cfg.TelegramToken,
			DefaultChatID: cfg.TelegramChatID,
			Groups:        cfg.Groups,
			UseThreadIDs:  cfg.UseThreadIDs,
			Frigate:       frigate,
		})
		if err != nil {
			log.Fatalf("Error initializing Telegram bot: %v", err)
		}
		tgBot.RegisterHandlers(ctx)
		go tgBot.Start(ctx)
		defer tgBot.Stop(ctx)

		deliveries, err := rabbit.Consume(cfg.RabbitMQQueue, "go-frigate-events")
		if err != nil {
			log.Fatalf("Error consuming RabbitMQ queue: %v", err)
		}
		defer rabbit.Close()
		frigateEvent := events.NewFrigateEvent(tgBot, cfg)

		for delivery := range deliveries {

			var eventT events.MQTTFrigateEvent
			err := json.Unmarshal(delivery.Body, &eventT)
			if err != nil {
				log.Printf("Error unmarshalling message: %v", err)
				delivery.Ack(false)
				continue
			}

			processing, err := redis.IsEventProcessing(ctx, eventT.After.ID, "video")
			if err != nil {
				log.Printf("Error checking if event is processing: %v", err)
				delivery.Ack(false)
				continue
			}

			if processing {
				delivery.Ack(false)
				continue
			}

			go func(event events.MQTTFrigateEvent) {
				redis.MarkEventAsProcessing(ctx, event.After.ID, "video")

				log.Printf("Received message: %s\n", event.Before.ID)

				processed, err := redis.IsEventProcessed(ctx, event.After.ID, "video")
				if err != nil {
					log.Fatalf("Error checking if event is processed: %v", err)
				}
				if processed {
					delivery.Ack(false)
					return
				}

				processed, err = frigateEvent.ProcessVideoEvent(ctx, event)
				if err != nil {
					log.Printf("Error processing video event: %v", err)
					delivery.Reject(true)
					return
				}
				if !processed {
					delivery.Ack(false)
					return
				}

				redis.MarkEventAsProcessed(ctx, event.After.ID, "video")
				delivery.Ack(false)
			}(eventT)
		}
	case "mqtt": //listen mqtt and publish to rabbit
		var mqttClient *mqtt_handler.MQTTClient

		mqttClient, err = mqtt_handler.NewClient(cfg.MQTTBroker, "go-frigate-events", cfg.MQTTUser, cfg.MQTTPassword)
		if err != nil {
			log.Fatalf("Error initializing MQTT client: %v", err)
		}

		if err := mqttClient.Subscribe(cfg.MQTTTopic, 1, func(c mqtt.Client, m mqtt.Message) {
			rabbit.Publish(ctx, cfg.RabbitMQExchange, cfg.RabbitMQRoutingKey, m.Payload())
			var event events.MQTTFrigateEvent
			err := json.Unmarshal(m.Payload(), &event)
			if err != nil {
				log.Printf("Error unmarshalling message: %v", err)
				return
			}
			log.Printf("Published message to RabbitMQ: %s\n", event.Before.ID)
		}); err != nil {
			log.Fatalf("Error subscribing to MQTT topic: %v", err)
		}
		defer mqttClient.Disconnect()

	default:
		log.Println("use ./frigate-events [telegram|mqtt]")
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

}
