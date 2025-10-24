package telegram_handler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/geffersonFerraz/frigate-events-telegram/config"
	"github.com/geffersonFerraz/frigate-events-telegram/frigate"
	"github.com/geffersonFerraz/frigate-events-telegram/redis_handler"
	tgbotapi "github.com/go-telegram/bot"
	"github.com/google/uuid"

	"github.com/go-telegram/bot/models"
)

// retryWithBackoff executes a function with exponential backoff retry
func retryWithBackoff(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > 30*time.Second {
				delay = 30 * time.Second // Cap at 30 seconds
			}

			log.Printf("Retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetries)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if it's a network error that might be temporary
		if isTemporaryError(err) {
			log.Printf("Temporary error on attempt %d: %v", attempt+1, err)
			continue
		}

		// If it's not a temporary error, don't retry
		return err
	}

	return fmt.Errorf("failed after %d attempts, last error: %w", maxRetries, lastErr)
}

// isTemporaryError checks if an error is likely temporary and worth retrying
func isTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary() || netErr.Timeout()
	}

	// Check for context timeout/cancellation
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false
	}

	// Check for specific error messages that indicate temporary issues
	errStr := strings.ToLower(err.Error())
	temporaryPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no route to host",
		"temporary failure",
		"deadline exceeded",
		"context deadline exceeded",
	}

	for _, pattern := range temporaryPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// TelegramBot encapsulates the Telegram bot functionality
type TelegramBot struct {
	Bot           *tgbotapi.Bot
	Token         string
	DefaultChatID int64
	Groups        []config.Group
	UseThreadIDs  bool
	StartTime     time.Time
	Redis         *redis_handler.RedisHandler
	Frigate       *frigate.Frigate
}

// Telegram interface defines the methods that a Telegram bot must implement
type Telegram interface {
	Start(ctx context.Context)
	RegisterHandlers(ctx context.Context)
	Stop(ctx context.Context) (bool, error)
	SendMessage(ctx context.Context, text string, cameraName string) error
	SendPhoto(ctx context.Context, photoBytes []byte, caption string, cameraName string) error
	SendVideo(ctx context.Context, videoBytes []byte, caption string, cameraName string) error
}

// NewBot creates a new instance of TelegramBot
func NewBot(config TelegramBot) (Telegram, error) {
	// Create bot with basic configuration
	botOptions := []tgbotapi.Option{
		tgbotapi.WithCheckInitTimeout(10 * time.Minute),
		tgbotapi.WithErrorsHandler(func(err error) {
			log.Printf("Error no tgBot: %v", err)
		}),
		tgbotapi.WithDebug(),
	}
	bot, err := tgbotapi.New(config.Token, botOptions...)
	if err != nil {
		return nil, err
	}

	cameraThreadIDs := make(map[string]int64)
	for _, group := range config.Groups {
		cameraThreadIDs[group.Name] = group.ID
	}

	tb := &TelegramBot{
		Token:         config.Token,
		DefaultChatID: config.DefaultChatID,
		Groups:        config.Groups,
		UseThreadIDs:  config.UseThreadIDs,
		StartTime:     time.Now(),
		Bot:           bot,
		Redis:         config.Redis,
		Frigate:       config.Frigate,
	}

	return tb, nil
}

// Start starts the Telegram bot
func (b *TelegramBot) Start(ctx context.Context) {
	b.Bot.Start(ctx)
}

// Stop stops the Telegram bot
func (b *TelegramBot) Stop(ctx context.Context) (bool, error) {
	return b.Bot.Close(ctx)
}

// RegisterHandlers registers handlers for the bot
func (b *TelegramBot) RegisterHandlers(ctx context.Context) {
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/status", tgbotapi.MatchTypePrefix, b.handleStatus)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/clean", tgbotapi.MatchTypePrefix, b.handleClean)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/restart", tgbotapi.MatchTypePrefix, b.handleRestart)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/help", tgbotapi.MatchTypePrefix, b.handleHelp)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/snapshot", tgbotapi.MatchTypePrefix, b.handleSnapshot)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/record", tgbotapi.MatchTypePrefix, b.handleRecord)
}

// SendMessage sends a text message to the specified chat
func (b *TelegramBot) SendMessage(ctx context.Context, text string, cameraName string) error {
	message := &tgbotapi.SendMessageParams{
		ChatID: b.DefaultChatID,
		Text:   text,
	}
	if b.UseThreadIDs {
		message.MessageThreadID = int(b.getChatID(cameraName))
	}

	// Use retry mechanism for sending messages
	return retryWithBackoff(ctx, 3, 2*time.Second, func() error {
		_, err := b.Bot.SendMessage(ctx, message)
		if err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}
		return nil
	})
}

// SendPhoto sends a photo to the specified chat
func (b *TelegramBot) SendPhoto(ctx context.Context, photoBytes []byte, caption string, cameraName string) error {
	photo := &models.InputMediaPhoto{
		Media:           "attach://" + uuid.New().String() + ".jpg",
		MediaAttachment: bytes.NewReader(photoBytes),
		Caption:         caption,
	}
	medias := []models.InputMedia{
		photo,
	}

	message := &tgbotapi.SendMediaGroupParams{
		ChatID: b.DefaultChatID,
		Media:  medias,
	}
	if b.UseThreadIDs {
		message.MessageThreadID = int(b.getChatID(cameraName))
	}

	// Use retry mechanism for sending photos
	return retryWithBackoff(ctx, 3, 2*time.Second, func() error {
		_, err := b.Bot.SendMediaGroup(ctx, message)
		if err != nil {
			return fmt.Errorf("error sending photo: %w", err)
		}
		return nil
	})
}

// SendVideo sends a video to the specified chat
func (b *TelegramBot) SendVideo(ctx context.Context, videoBytes []byte, caption string, cameraName string) error {
	video := &models.InputMediaVideo{
		Media:           "attach://" + uuid.New().String() + ".mp4",
		MediaAttachment: bytes.NewReader(videoBytes),
		Caption:         caption,
	}

	medias := []models.InputMedia{
		video,
	}

	message := &tgbotapi.SendMediaGroupParams{
		ChatID: b.DefaultChatID,
		Media:  medias,
	}
	if b.UseThreadIDs {
		message.MessageThreadID = int(b.getChatID(cameraName))
	}

	// Use retry mechanism for sending videos
	return retryWithBackoff(ctx, 3, 2*time.Second, func() error {
		_, err := b.Bot.SendMediaGroup(ctx, message)
		if err != nil {
			return fmt.Errorf("error sending video: %w", err)
		}
		return nil
	})
}

// getChatID returns the chat ID for a specific camera
func (b *TelegramBot) getChatID(cameraName string) int64 {
	for _, group := range b.Groups {
		if group.Name == cameraName {
			return group.ID
		}
	}
	// If no specific group is found for the camera, use the default group
	log.Printf("Warning: Group not found for camera '%s', using default group", cameraName)
	return b.DefaultChatID
}

// getCameraName returns the camera name for a specific chat ID
func (b *TelegramBot) getCameraName(chatID int64) string {
	for _, group := range b.Groups {
		if group.ID == chatID {
			return group.Name
		}
	}
	return ""
}

// handleStatus handles the /status command and returns system information
func (b *TelegramBot) handleStatus(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	// Get memory statistics
	memoryUsage := runtime.MemStats{}
	runtime.ReadMemStats(&memoryUsage)

	// Format memory usage in MB
	memoryMB := float64(memoryUsage.TotalAlloc) / (1024 * 1024)

	// Get CPU usage
	cpuUsage := runtime.NumCPU()

	// Format uptime
	uptime := time.Since(b.StartTime)
	uptimeStr := formatDuration(uptime)

	statusInfo := []string{
		"✅ System running",
		fmt.Sprintf("🕒 Uptime: %s", uptimeStr),
		fmt.Sprintf("💻 Memory usage: %.2f MB", memoryMB),
		fmt.Sprintf("💻 Available CPU cores: %d", cpuUsage),
	}

	cameraName := b.getCameraName(update.Message.Chat.ID)

	if cameraName != "" {
		statusInfo = append(statusInfo, fmt.Sprintf("📷 Selected camera: %s", cameraName))
	}

	message := &tgbotapi.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   strings.Join(statusInfo, "\n"),
	}
	if update.Message.MessageThreadID != 0 {
		message.MessageThreadID = int(update.Message.MessageThreadID)
	}
	bot.SendMessage(ctx, message)
}

// formatDuration formats a duration in a more readable format
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes, %d seconds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%d minutes, %d seconds", minutes, seconds)
	}
	return fmt.Sprintf("%d seconds", seconds)
}

// handleClean handles the /clean command and clears Redis data
func (b *TelegramBot) handleClean(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	// Clear everything from Redis
	b.Redis.FlushAll(ctx)
	bot.SendMessage(ctx, stringToMessage("Redis cleared successfully!", update.Message.Chat.ID, &update.Message.MessageThreadID))
}

// handleRestart handles the /restart command and restarts the bot
func (b *TelegramBot) handleRestart(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	bot.SendMessage(ctx, stringToMessage("Restarting the bot...", update.Message.Chat.ID, &update.Message.MessageThreadID))
	os.Exit(0)
}

// handleHelp handles the /help command and shows available commands
func (b *TelegramBot) handleHelp(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	commands := []string{
		"🔄 /restart - Restarts the bot",
		"📸 /snapshot - Takes a snapshot from the camera of the current thread",
		"🧹 /clean - Clears temporary data",
		"ℹ️ /status - Shows system status",
		"❓ /help - Shows this help message",
		"🎥 /record [seconds] - Creates a recording event for the camera of the current thread",
	}

	bot.SendMessage(ctx, stringToMessage(strings.Join(commands, "\n"), update.Message.Chat.ID, &update.Message.MessageThreadID))
}

// stringToMessage creates a SendMessageParams from text, chat ID, and optional message thread ID
func stringToMessage(text string, chatID int64, messageThreadID *int) *tgbotapi.SendMessageParams {
	message := &tgbotapi.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}
	if messageThreadID != nil {
		message.MessageThreadID = int(*messageThreadID)
	}
	return message
}

// handleSnapshot handles the /snapshot command and sends a camera snapshot
func (b *TelegramBot) handleSnapshot(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	cameraName := b.getCameraName(int64(update.Message.MessageThreadID))
	if cameraName == "" {
		bot.SendMessage(ctx, stringToMessage("No camera selected", update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}

	snapshot, err := b.Frigate.GetSnapshot(ctx, cameraName)
	if err != nil {
		bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Error getting snapshot: %v", err), update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}

	b.SendPhoto(ctx, snapshot, fmt.Sprintf("Snapshot from camera %s", cameraName), cameraName)
}

// handleRecord handles the /record command and creates a recording event
func (b *TelegramBot) handleRecord(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	cameraName := b.getCameraName(int64(update.Message.MessageThreadID))
	if cameraName == "" {
		bot.SendMessage(ctx, stringToMessage("No camera selected", update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}

	duration := 10
	var err error
	if update.Message.Text != "/record" {
		duration, err = strconv.Atoi(strings.Split(update.Message.Text, " ")[1])
		if err != nil {
			bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Error converting time: %v", err), update.Message.Chat.ID, &update.Message.MessageThreadID))
			return
		}
	}

	_, err = b.Frigate.CreateEvent(ctx, cameraName, duration)
	if err != nil {
		bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Error creating event: %v", err), update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}
	bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Event created successfully, wait for the recording to be processed"), update.Message.Chat.ID, &update.Message.MessageThreadID))
}
