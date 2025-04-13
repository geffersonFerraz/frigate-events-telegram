package frigate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Frigate struct {
	URL string
}

func NewFrigate(url string) *Frigate {
	return &Frigate{URL: url}
}
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

type EventResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	EventID string `json:"event_id"`
}

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
