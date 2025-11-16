package events

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/geffersonFerraz/frigate-events/config"
	telegram_handler "github.com/geffersonFerraz/frigate-events/telegram"
)

type frigateEvent struct {
	tgBot telegram_handler.Telegram
	cfg   *config.Config
}

type FrigateEvent interface {
	downloadVideo(ctx context.Context, clipURL string, maxRetries int) ([]byte, error)
	ProcessVideoEvent(ctx context.Context, event MQTTFrigateEvent) (bool, error)
	hasEndtimeWithClip(event MQTTFrigateEvent) bool
}

func NewFrigateEvent(tgBot telegram_handler.Telegram, cfg *config.Config) FrigateEvent {
	return &frigateEvent{
		tgBot: tgBot,
		cfg:   cfg,
	}
}

func (h *frigateEvent) downloadVideo(ctx context.Context, clipURL string, maxRetries int) ([]byte, error) {
	var videoBytes []byte
	var lastErr error
	req, err := http.NewRequestWithContext(ctx, "GET", clipURL, nil)
	if err != nil {
		lastErr = fmt.Errorf("error creating request: %w", err)
		return nil, lastErr
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		lastErr = fmt.Errorf("error fetching clip: %w", err)
		return nil, lastErr
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("status code %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, lastErr
	}

	videoBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		lastErr = fmt.Errorf("error reading clip bytes: %w", err)
		return nil, lastErr
	}

	if len(videoBytes) > 0 {
		return videoBytes, nil
	}

	lastErr = fmt.Errorf("empty clip received")

	return nil, fmt.Errorf("failed after %d attempts: %v", maxRetries, lastErr)
}

func (h *frigateEvent) hasEndtimeWithClip(event MQTTFrigateEvent) bool {
	return event.After.EndTime != nil && event.After.EndTime.(float64) > 0
}

// processVideoEvent processes the download and sending of the video in a separate goroutine
func (h *frigateEvent) ProcessVideoEvent(ctx context.Context, event MQTTFrigateEvent) (bool, error) {
	// Create a context with timeout for the entire process
	videoCtx, videoCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer videoCancel()

	// Channel to receive the processing result
	resultChan := make(chan error, 1)

	go func() {
		clipURL := fmt.Sprintf("%s/api/events/%s/clip.mp4", strings.TrimSuffix(h.cfg.FrigateURL, "/"), event.After.ID)

		videoBytes, err := h.downloadVideo(videoCtx, clipURL, 9) // 3 attempts
		if err != nil {
			resultChan <- fmt.Errorf("failed to download video: %w", err)
			return
		}

		// if videoBytes > 49mb, split using first 49mb and send it
		if len(videoBytes) > 49*1024*1024 {
			videoBytes = videoBytes[:49*1024*1024]
		}

		if event.After.SubLabel != nil {
			for _, subLabel := range event.After.SubLabel {
				event.After.Label = fmt.Sprintf("%s (%v)", event.After.Label, subLabel)
			}
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
			return false, err
		} else {
			log.Printf("Clip for event %s sent to Telegram successfully.", event.After.ID)
			return true, nil
		}
	case <-videoCtx.Done():
		log.Printf("Timeout processing video for event %s: %v", event.After.ID, videoCtx.Err())
		return false, fmt.Errorf("timeout processing video for event %s: %v", event.After.ID, videoCtx.Err())
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
