package telegram_handler

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/geffersonFerraz/frigate-events-telegram/config"
	tgbotapi "github.com/go-telegram/bot"
	"github.com/google/uuid"

	"github.com/go-telegram/bot/models"
)

// TelegramBot encapsula a funcionalidade do bot do Telegram
type TelegramBot struct {
	bot            *tgbotapi.Bot
	chatIDs        []config.Group // Mapeia nomes de câmeras para IDs de chat
	commandHandler *CommandHandler
	defaultChatID  int64
}

// TelegramConfig contém as configurações para o bot do Telegram
type TelegramConfig struct {
	Token         string
	DefaultChatID int64
	Groups        []config.Group
	UseThreadIDs  bool
}

// NewBot cria uma nova instância do TelegramBot
func NewBot(config TelegramConfig) (*TelegramBot, error) {
	bot, err := tgbotapi.New(config.Token)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar bot: %w", err)
	}

	tb := &TelegramBot{
		bot:           bot,
		chatIDs:       config.Groups,
		defaultChatID: config.DefaultChatID,
	}

	// Inicializar o gerenciador de comandos
	cmdHandler := NewCommandHandler(CommandConfig{
		Bot:          bot,
		ChatIDs:      config.Groups,
		UseThreadIDs: config.UseThreadIDs,
	})
	tb.commandHandler = cmdHandler

	return tb, nil
}

// Start inicia o bot do Telegram
func (b *TelegramBot) Start(ctx context.Context) {
	// Iniciar o gerenciador de comandos
	b.commandHandler.Start(ctx)
}

// Stop para o bot do Telegram
func (b *TelegramBot) Stop() {
	// Parar o gerenciador de comandos
	b.commandHandler.Stop()
}

// EnableThreadIDs ativa ou desativa o uso de messageThreadID
func (b *TelegramBot) EnableThreadIDs(enabled bool) {
	b.commandHandler.EnableThreadIDs(enabled)
}

// ProcessUpdate processa uma atualização do Telegram
func (b *TelegramBot) ProcessUpdate(ctx context.Context, update *models.Update) {
	// Processar comandos
	b.commandHandler.ProcessUpdate(ctx, update)
}

// RegisterCameraThreadID associa um messageThreadID a uma câmera
func (b *TelegramBot) RegisterCameraThreadID(threadID int64, cameraName string) {
	b.commandHandler.RegisterCameraThreadID(threadID, cameraName)
}

// SendMessage envia uma mensagem de texto para o chat especificado
func (b *TelegramBot) SendMessage(ctx context.Context, text string, cameraName string) error {
	message := &tgbotapi.SendMessageParams{
		ChatID: b.defaultChatID,
		Text:   text,
	}
	if b.commandHandler.useThreadIDs {
		message.MessageThreadID = int(b.getChatID(cameraName))
	}
	_, err := b.bot.SendMessage(ctx, message)
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
		ChatID: b.defaultChatID,
		Media:  medias,
	}
	if b.commandHandler.useThreadIDs {
		message.MessageThreadID = int(b.getChatID(cameraName))
	}

	_, err := b.bot.SendMediaGroup(ctx, message)
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
		ChatID: b.defaultChatID,
		Media:  medias,
	}
	if b.commandHandler.useThreadIDs {
		message.MessageThreadID = int(b.getChatID(cameraName))
	}

	_, err := b.bot.SendMediaGroup(ctx, message)
	if err != nil {
		return fmt.Errorf("erro ao enviar vídeo: %w", err)
	}

	return nil
}

// getChatID retorna o ID do chat para uma câmera específica
func (b *TelegramBot) getChatID(cameraName string) int64 {
	for _, group := range b.chatIDs {
		if group.Name == cameraName {
			return group.ID
		}
	}
	// Se não encontrar um grupo específico para a câmera, usar o grupo padrão
	log.Printf("Aviso: Grupo não encontrado para câmera '%s', usando grupo padrão", cameraName)
	return b.defaultChatID
}

// SetupWebhook configura um webhook para receber atualizações do Telegram
func (b *TelegramBot) SetupWebhook(ctx context.Context, webhookURL string, port int) error {
	// Configurar o webhook
	_, err := b.bot.SetWebhook(ctx, &tgbotapi.SetWebhookParams{
		URL: webhookURL,
	})
	if err != nil {
		return fmt.Errorf("erro ao configurar webhook: %w", err)
	}
	log.Printf("Webhook configurado para %s", webhookURL)
	return nil
}
