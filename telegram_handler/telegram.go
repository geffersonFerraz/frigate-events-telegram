package telegram_handler

import (
	"bytes"
	"context"
	"fmt"
	"log"
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

// TelegramBot encapsula a funcionalidade do bot do Telegram
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

type Telegram interface {
	Start(ctx context.Context)
	RegisterHandlers(ctx context.Context)
	Stop(ctx context.Context) (bool, error)
	SendMessage(ctx context.Context, text string, cameraName string) error
	SendPhoto(ctx context.Context, photoBytes []byte, caption string, cameraName string) error
	SendVideo(ctx context.Context, videoBytes []byte, caption string, cameraName string) error
}

// NewBot cria uma nova instância do TelegramBot
func NewBot(config TelegramBot) (Telegram, error) {
	bot, err := tgbotapi.New(config.Token)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar bot: %w", err)
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

func (b *TelegramBot) Start(ctx context.Context) {
	b.Bot.Start(ctx)
}

func (b *TelegramBot) Stop(ctx context.Context) (bool, error) {
	return b.Bot.Close(ctx)
}

// RegisterHandler registra um handler para o bot
func (b *TelegramBot) RegisterHandlers(ctx context.Context) {
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/status", tgbotapi.MatchTypePrefix, b.handleStatus)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/clean", tgbotapi.MatchTypePrefix, b.handleClean)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/restart", tgbotapi.MatchTypePrefix, b.handleRestart)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/help", tgbotapi.MatchTypePrefix, b.handleHelp)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/snapshot", tgbotapi.MatchTypePrefix, b.handleSnapshot)
	b.Bot.RegisterHandler(tgbotapi.HandlerTypeMessageText, "/record", tgbotapi.MatchTypePrefix, b.handleRecord)
}

// SendMessage envia uma mensagem de texto para o chat especificado
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
		return fmt.Errorf("erro ao enviar mensagem: %w", err)
	}
	return nil
}

// SendPhoto envia uma foto para o chat especificado
func (b *TelegramBot) SendPhoto(ctx context.Context, photoBytes []byte, caption string, cameraName string) error {
	photo := &models.InputMediaPhoto{
		Media:           "attach://" + uuid.New().String(),
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
		return fmt.Errorf("erro ao enviar foto: %w", err)
	}
	return nil
}

// SendVideo envia um vídeo para o chat especificado
func (b *TelegramBot) SendVideo(ctx context.Context, videoBytes []byte, caption string, cameraName string) error {
	video := &models.InputMediaVideo{
		Media:           "attach://" + uuid.New().String(),
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
		return fmt.Errorf("erro ao enviar vídeo: %w", err)
	}

	return nil
}

// getChatID retorna o ID do chat para uma câmera específica
func (b *TelegramBot) getChatID(cameraName string) int64 {
	for _, group := range b.Groups {
		if group.Name == cameraName {
			return group.ID
		}
	}
	// Se não encontrar um grupo específico para a câmera, usar o grupo padrão
	log.Printf("Aviso: Grupo não encontrado para câmera '%s', usando grupo padrão", cameraName)
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
	// Obter estatísticas de memória
	memoryUsage := runtime.MemStats{}
	runtime.ReadMemStats(&memoryUsage)

	// Formatar uso de memória em MB
	memoryMB := float64(memoryUsage.TotalAlloc) / (1024 * 1024)

	// Obter uso de CPU
	cpuUsage := runtime.NumCPU()

	// Formatar tempo de atividade
	uptime := time.Since(b.StartTime)
	uptimeStr := formatDuration(uptime)

	statusInfo := []string{
		"✅ Sistema em execução",
		fmt.Sprintf("🕒 Tempo de atividade: %s", uptimeStr),
		fmt.Sprintf("💻 Uso de memória: %.2f MB", memoryMB),
		fmt.Sprintf("💻 Núcleos de CPU disponíveis: %d", cpuUsage),
	}

	cameraName := b.getCameraName(update.Message.Chat.ID)

	if cameraName != "" {
		statusInfo = append(statusInfo, fmt.Sprintf("📷 Câmera selecionada: %s", cameraName))
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

// formatDuration formata uma duração em um formato mais legível
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%d dias, %d horas, %d minutos", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d horas, %d minutos, %d segundos", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%d minutos, %d segundos", minutes, seconds)
	}
	return fmt.Sprintf("%d segundos", seconds)
}

func (b *TelegramBot) handleClean(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	// limpa tudo que for do redis
	b.Redis.FlushAll(ctx)
	bot.SendMessage(ctx, stringToMessage("Redis limpo com sucesso!", update.Message.Chat.ID, &update.Message.MessageThreadID))
}

func (b *TelegramBot) handleRestart(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	bot.SendMessage(ctx, stringToMessage("Reiniciando o bot...", update.Message.Chat.ID, &update.Message.MessageThreadID))
	os.Exit(0)
}

func (b *TelegramBot) handleHelp(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	commands := []string{
		"🔄 /restart - Reinicia o bot",
		"📸 /snapshot - Tira um snapshot da câmera da thread atual",
		"🧹 /clean - Limpa dados temporários",
		"ℹ️ /status - Mostra o status do sistema",
		"❓ /help - Mostra esta mensagem de ajuda",
		"🎥 /record [segundos]- Cria um evento de gravação da câmera da thread atual",
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
		bot.SendMessage(ctx, stringToMessage("Nenhuma câmera selecionada", update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}

	snapshot, err := b.Frigate.GetSnapshot(ctx, cameraName)
	if err != nil {
		bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Erro ao obter snapshot: %v", err), update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}

	b.SendPhoto(ctx, snapshot, fmt.Sprintf("Snapshot da câmera %s", cameraName), cameraName)
}

func (b *TelegramBot) handleRecord(ctx context.Context, bot *tgbotapi.Bot, update *models.Update) {
	cameraName := b.getCameraName(int64(update.Message.MessageThreadID))
	if cameraName == "" {
		bot.SendMessage(ctx, stringToMessage("Nenhuma câmera selecionada", update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}

	duration := 10
	var err error
	if update.Message.Text != "/record" {
		duration, err = strconv.Atoi(strings.Split(update.Message.Text, " ")[1])
		if err != nil {
			bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Erro ao converter tempo: %v", err), update.Message.Chat.ID, &update.Message.MessageThreadID))
			return
		}
	}

	_, err = b.Frigate.CreateEvent(ctx, cameraName, duration)
	if err != nil {
		bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Erro ao criar evento: %v", err), update.Message.Chat.ID, &update.Message.MessageThreadID))
		return
	}
	bot.SendMessage(ctx, stringToMessage(fmt.Sprintf("Evento criado com sucesso, aguarde a gravação ser processada"), update.Message.Chat.ID, &update.Message.MessageThreadID))
}
