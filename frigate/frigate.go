package frigate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Frigate represents a Frigate NVR instance with its base URL
type Frigate struct {
	URL string
}

// NewFrigate creates a new Frigate instance with the specified URL
func NewFrigate(url string) *Frigate {
	return &Frigate{URL: url}
}

// GetSnapshot retrieves the latest snapshot from a specific camera
func (f *Frigate) GetSnapshot(ctx context.Context, camera string) ([]byte, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/%s/latest.jpg", f.URL, camera))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// EventResponse represents the response structure from Frigate API when creating events
type EventResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	EventID string `json:"event_id"`
}

// CreateEvent creates a new recording event for a specific camera with the given duration
func (f *Frigate) CreateEvent(ctx context.Context, camera string, durationSeconds int) (string, error) {
	body := fmt.Sprintf(`{"duration": %d, "source_type": "telegram", "sub_label": "telegram", "score": 0, "include_recording": true, "draw": {}}`, durationSeconds)
	resp, err := http.Post(fmt.Sprintf("%s/api/events/%s/telegram/create", f.URL, camera), "application/json", bytes.NewBuffer([]byte(body)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// response
	// {
	// 	"success": true,
	// 	"message": "string",
	// 	"event_id": "string"
	//   }
	var data EventResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", err
	}
	return data.EventID, nil
}
