package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	// Removed tgbot import on purpose as it's not used directly in main now

	"github.com/geffersonFerraz/frigate-events-telegram/config" // Relative import for go module
	"github.com/geffersonFerraz/frigate-events-telegram/frigate"
	"github.com/geffersonFerraz/frigate-events-telegram/mqtt_handler"
	"github.com/geffersonFerraz/frigate-events-telegram/redis_handler"
	"github.com/geffersonFerraz/frigate-events-telegram/telegram_handler"
)

// FrigateEvent represents the basic structure of a Frigate event (may need more fields)
type FrigateEvent struct {
	Before struct {
		ID          string  `json:"id"`
		Label       string  `json:"label"`
		Camera      string  `json:"camera"`
		StartTime   float64 `json:"start_time"`
		HasSnapshot bool    `json:"has_snapshot"`
		HasClip     bool    `json:"has_clip"`
	} `json:"before"`
	After struct {
		ID          string  `json:"id"`
		Label       string  `json:"label"`
		Camera      string  `json:"camera"`
		StartTime   float64 `json:"start_time"`
		HasSnapshot bool    `json:"has_snapshot"`
		HasClip     bool    `json:"has_clip"`
	} `json:"after"`
	Type string `json:"type"` // "new", "update", "end"
}

// AppHandler contains the necessary dependencies for the MQTT handler
type AppHandler struct {
	tgBot      telegram_handler.Telegram
	cfg        *config.Config
	httpClient *http.Client // For fetching images
	redis      *redis_handler.RedisHandler
}

// newAppHandler creates a new instance of AppHandler
func newAppHandler(bot telegram_handler.Telegram, cfg *config.Config, redis *redis_handler.RedisHandler) *AppHandler {
	return &AppHandler{
		tgBot:      bot,
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second}, // 10s timeout for fetching images
		redis:      redis,
	}
}

// downloadVideo attempts to download the video from Frigate, with retry if necessary
func (h *AppHandler) downloadVideo(ctx context.Context, clipURL string, maxRetries int) ([]byte, error) {
	var videoBytes []byte
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("Attempt %d of %d to download clip %s", attempt, maxRetries, clipURL)
			// Wait a bit before retrying
			time.Sleep(2 * time.Second)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", clipURL, nil)
		if err != nil {
			lastErr = fmt.Errorf("error creating request: %w", err)
			continue
		}

		resp, err := h.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error fetching clip: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("status code %d: %s", resp.StatusCode, string(bodyBytes))
			continue
		}

		videoBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("error reading clip bytes: %w", err)
			continue
		}

		if len(videoBytes) > 0 {
			return videoBytes, nil
		}

		lastErr = fmt.Errorf("empty clip received")
	}

	return nil, fmt.Errorf("failed after %d attempts: %v", maxRetries, lastErr)
}

// processVideoEvent processes the download and sending of the video in a separate goroutine
func (h *AppHandler) processVideoEvent(ctx context.Context, event FrigateEvent, clipURL string) {
	// Create a context with timeout for the entire process
	videoCtx, videoCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer videoCancel()

	// Channel to receive the processing result
	resultChan := make(chan error, 1)

	go func() {
		// Try to download the video (with retry)
		videoBytes, err := h.downloadVideo(videoCtx, clipURL, 9) // 3 attempts
		if err != nil {
			resultChan <- fmt.Errorf("failed to download video: %w", err)
			return
		}

		// if videoBytes > 49mb, split using first 49mb and send it
		if len(videoBytes) > 49*1024*1024 {
			videoBytes = videoBytes[:49*1024*1024]
		}

		// Create caption for the video
		caption := fmt.Sprintf("🎬 #%s\n🎥 %s\n🕒 %s\n🔗 #%s",
			event.After.Label,
			event.After.Camera,
			time.Unix(int64(event.After.StartTime), 0).Add(time.Duration(h.cfg.TimezoneAjust)*time.Hour).Format("02/01/2006 15:04:05"),
			formatStringID(event.After.ID))

		log.Printf("Attempting to send clip for event %s (%d bytes) to Telegram...", event.After.ID, len(videoBytes))

		// Send video via Telegram
		if err := h.tgBot.SendVideo(videoCtx, videoBytes, caption, event.After.Camera); err != nil {
			resultChan <- fmt.Errorf("error sending video: %w", err)
			return
		}

		resultChan <- nil
	}()

	// Wait for result or timeout
	select {
	case err := <-resultChan:
		if err != nil {
			log.Printf("Error processing video for event %s: %v", event.After.ID, err)
		} else {
			log.Printf("Clip for event %s sent to Telegram successfully.", event.After.ID)
		}
	case <-videoCtx.Done():
		log.Printf("Timeout processing video for event %s: %v", event.After.ID, videoCtx.Err())
	}
}

// formatStringID formats the event ID by removing hyphens and extracting the part after the dot
func formatStringID(id string) string {
	id = strings.ReplaceAll(id, "-", "")
	result := strings.Split(id, ".")
	if len(result) > 1 {
		return result[1]
	}
	return id
}

// handleMQTTMessage is the method that processes MQTT messages
func (h *AppHandler) handleMQTTMessage(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received: %s from topic: %s\n", msg.Payload(), msg.Topic())

	var event FrigateEvent
	if err := json.Unmarshal(msg.Payload(), &event); err != nil {
		log.Printf("Error decoding JSON from event: %v", err)
		return
	}

	// Check if the event has already been processed
	ctx := context.Background()
	processed, err := h.redis.IsEventProcessed(ctx, event.After.ID, event.Type)
	if err != nil {
		log.Printf("Error checking event in Redis: %v", err)
		return
	}
	if processed {
		log.Printf("Event %s (type: %s) has already been processed previously, ignoring.", event.After.ID, event.Type)
		return
	}

	// We want to send only for new or updated events that have snapshots
	if (event.Type == "new" || event.Type == "update") && event.After.HasSnapshot {
		log.Printf("Processing event '%s' for camera '%s' (ID: %s)", event.After.Label, event.After.Camera, event.After.ID)

		// Build snapshot URL
		snapshotURL := fmt.Sprintf("%s/api/events/%s/snapshot.jpg", strings.TrimSuffix(h.cfg.FrigateURL, "/"), event.After.ID)

		// Download the image
		req, err := http.NewRequestWithContext(context.Background(), "GET", snapshotURL, nil)
		if err != nil {
			log.Printf("Error creating request for snapshot %s: %v", snapshotURL, err)
			return
		}

		resp, err := h.httpClient.Do(req)
		if err != nil {
			log.Printf("Error fetching snapshot %s: %v", snapshotURL, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Error fetching snapshot %s: Status %d", snapshotURL, resp.StatusCode)
			bodyBytes, _ := io.ReadAll(resp.Body)
			log.Printf("Response body: %s", string(bodyBytes))
			return
		}

		imgBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading bytes from snapshot %s: %v", snapshotURL, err)
			return
		}

		// Create caption for the photo
		caption := fmt.Sprintf("🖼️ #%s\n🎥 %s\n🕒 %s\n🔗 #%s",
			event.After.Label,
			event.After.Camera,
			time.Unix(int64(event.After.StartTime), 0).Add(time.Duration(h.cfg.TimezoneAjust)*time.Hour).Format("02/01/2006 15:04:05"),
			formatStringID(event.After.ID))

		// Send photo via Telegram
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.tgBot.SendPhoto(ctx, imgBytes, caption, event.After.Camera); err != nil {
			log.Printf("Error sending photo to Telegram: %v", err)
		}
		log.Printf("Photo for event %s sent to Telegram.", event.After.ID)

		// Mark event as processed after sending the photo
		if err := h.redis.MarkEventAsProcessed(ctx, event.After.ID, event.Type); err != nil {
			log.Printf("Error marking event as processed in Redis: %v", err)
		}

	} else if event.Type == "end" && event.After.HasClip {
		log.Printf("Processing end of event '%s' for camera '%s' (ID: %s) - Sending clip.", event.After.Label, event.After.Camera, event.After.ID)

		// Build clip URL
		clipURL := fmt.Sprintf("%s/api/events/%s/clip.mp4", strings.TrimSuffix(h.cfg.FrigateURL, "/"), event.After.ID)

		// Process the video in a separate goroutine
		go h.processVideoEvent(context.Background(), event, clipURL)

		// Mark event as processed after starting video processing
		if err := h.redis.MarkEventAsProcessed(ctx, event.After.ID, event.Type); err != nil {
			log.Printf("Error marking event as processed in Redis: %v", err)
		}

	} else {
		// log.Printf("Event ignored (Type: %s, Snapshot: %t, Clip: %t)", event.Type, event.After.HasSnapshot, event.After.HasClip)
	}
}

func main() {
	fmt.Println("Starting Frigate Events Telegram...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Initialize Redis
	redis, err := redis_handler.NewRedisHandler(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Error initializing Redis: %v", err)
	}
	defer redis.Close()

	// Initialize Frigate
	frigate := frigate.NewFrigate(cfg.FrigateURL)

	// Initialize Telegram bot
	tgBot, err := telegram_handler.NewBot(telegram_handler.TelegramBot{
		Token:         cfg.TelegramToken,
		DefaultChatID: cfg.TelegramChatID,
		Groups:        cfg.Groups,
		UseThreadIDs:  cfg.UseThreadIDs,
		Redis:         redis,
		Frigate:       frigate,
	})
	if err != nil {
		log.Fatalf("Error initializing Telegram bot: %v", err)
	}
	ctx := context.Background()
	// Start Telegram command processing
	tgBot.RegisterHandlers(ctx)
	go tgBot.Start(ctx)
	defer tgBot.Stop(ctx)

	var mqttClient *mqtt_handler.MQTTClient

	// Initialize MQTT client
	if !cfg.CheckTelegram {
		mqttClient, err = mqtt_handler.NewClient(cfg.MQTTBroker, "frigate-event-listener", cfg.MQTTUser, cfg.MQTTPassword)
		if err != nil {
			log.Fatalf("Error initializing MQTT client: %v", err)
		}
	}

	// Create the application handler
	appHandler := newAppHandler(tgBot, cfg, redis)
	if !cfg.CheckTelegram {
		// Subscribe to Frigate events topic using the handler method
		if err := mqttClient.Subscribe(cfg.MQTTTopic, 1, appHandler.handleMQTTMessage); err != nil {
			log.Fatalf("Error subscribing to MQTT topic: %v", err)
		}
	}

	// Send startup message to Telegram
	startupMessage := "✅ Frigate Events Telegram bot initialized successfully! Waiting for events..."
	if cfg.CheckTelegram {
		startupMessage = "🔴 Debug operation without camera integration."
	}
	if err := tgBot.SendMessage(context.Background(), startupMessage, "General"); err != nil {
		log.Printf("Warning: Failed to send startup message to Telegram: %v", err)
	}

	fmt.Println("Application ready. Waiting for MQTT events...")

	// Wait for interrupt signal to finish
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down...")
	if !cfg.CheckTelegram {
		mqttClient.Disconnect()
	}
}
