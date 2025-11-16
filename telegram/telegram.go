package telegram_handler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/geffersonFerraz/frigate-events/config"
	"github.com/geffersonFerraz/frigate-events/frigate"
	tgbotapi "github.com/go-telegram/bot"
	"github.com/google/uuid"

	"github.com/go-telegram/bot/models"
)

type TelegramBot struct {
	Bot           *tgbotapi.Bot
	Token         string
	DefaultChatID int64
	Groups        []config.Group
	UseThreadIDs  bool
	StartTime     time.Time
	Frigate       *frigate.Frigate
}

type Telegram interface {
	Start(ctx context.Context)
	RegisterHandlers(ctx context.Context)
	Stop(ctx context.Context) (bool, error)
	SendMessage(ctx context.Context, text string, cameraName string) error
	SendPhoto(ctx context.Context, photoBytes []byte, caption string, cameraName string) error
	SendVideo(ctx context.Context, videoBytes []byte, caption string, cameraName string) error
}

func NewBot(config TelegramBot) (Telegram, error) {
	botOptions := []tgbotapi.Option{
		tgbotapi.WithWorkers(4),
		tgbotapi.WithCheckInitTimeout(10 * time.Minute),
		tgbotapi.WithErrorsHandler(func(err error) {
			panic(err)
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
		Frigate:       config.Frigate,
	}

	return tb, nil
}

func (b *TelegramBot) Start(ctx context.Context) {
	b.Bot.Start(ctx)
}

func (b *TelegramBot) Stop(ctx context.Context) (bool, error) {
	return b.Bot.Close(ctx)
}

func (b *TelegramBot) Restart(ctx context.Context) {
	b.Bot.Close(ctx)
	b.Bot.Start(ctx)
}

func (b *TelegramBot) RegisterHandlers(ctx context.Context) {
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/status", tgbotapi.MatchTypePrefix, b.handleStatus)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/restart", tgbotapi.MatchTypePrefix, b.handleRestart)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/help", tgbotapi.MatchTypePrefix, b.handleHelp)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/snapshot", tgbotapi.MatchTypePrefix, b.handleSnapshot)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/record", tgbotapi.MatchTypePrefix, b.handleRecord)
}

func (b *TelegramBot) SendMessage(ctx context.Context, text string, cameraName string) error {
	message := &tgbotapi.SendMessageParams{
		ChatID: b.DefaultChatID,
		Text:   text,
	}
	if b.UseThreadIDs {
		message.MessageThreadID = int(b.getChatID(cameraName))
	}

	_, err := b.Bot.SendMessage(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}
	return nil

}

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

	_, err := b.Bot.SendMediaGroup(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending photo: %w", err)
	}
	return nil

}

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

	_, err := b.Bot.SendMediaGroup(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending video: %w", err)
	}
	return nil

}

func (b *TelegramBot) getChatID(cameraName string) int64 {
	for _, group := range b.Groups {
		if group.Name == cameraName {
			return group.ID
		}
	}
	log.Printf("Warning: Group not found for camera '%s', using default group", cameraName)
	return b.DefaultChatID
}

func (b *TelegramBot) getCameraName(chatID int64) string {
	for _, group := range b.Groups {
		if group.ID == chatID {
			return group.Name
		}
	}
	return ""
}

func (b *TelegramBot) handleStatus(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	memoryUsage := runtime.MemStats{}
	runtime.ReadMemStats(&memoryUsage)
	memoryMB := float64(memoryUsage.TotalAlloc) / (1024 * 1024)
	cpuUsage := runtime.NumCPU()
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

func (b *TelegramBot) handleRestart(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	bot.SendMessage(ctx, stringToMessage("Restarting the bot...", update.Message.Chat.ID, &update.Message.MessageThreadID))
	os.Exit(0)
}

func (b *TelegramBot) handleHelp(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	commands := []string{
		"🔄 /restart - Restarts the bot",
		"📸 /snapshot - Takes a snapshot from the camera of the current thread",
		"ℹ️ /status - Shows system status",
		"❓ /help - Shows this help message",
		"🎥 /record [seconds] - Creates a recording event for the camera of the current thread",
	}

	bot.SendMessage(ctx, stringToMessage(strings.Join(commands, "\n"), update.Message.Chat.ID, &update.Message.MessageThreadID))
}

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

func (b *TelegramBot) handleRecord(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	cameraName := b.getCameraName(int64(update.Message.MessageThreadID))
	if cameraName == "" {
		bot.SendMessage(ctx, stringToMessage("No camera selected", update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}

	duration := 10
	var err error
	if update.Message.Text != "/record" {
		duration = int(5 * time.Second)
	}

	_, err = b.Frigate.CreateEvent(ctx, cameraName, duration)
	if err != nil {
		bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Error creating event: %v", err), update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}
	bot.SendMessage(ctx, stringToMessage("Event created successfully, wait for the recording to be processed", update.Message.Chat.ID, &update.Message.MessageThreadID))
}
