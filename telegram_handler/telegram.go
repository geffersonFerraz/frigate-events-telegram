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
	bot             *tgbotapi.Bot
	chatID          int64
	messageThreadID map[string]int // Mapeia nomes de câmeras para IDs de chat
}

// NewBot cria uma nova instância do TelegramBot
func NewBot(token string, defaultChatID int64, groups []config.Group) (*TelegramBot, error) {
	bot, err := tgbotapi.New(token)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar bot: %w", err)
	}

	// Criar mapa de chat IDs
	messageThreadID := make(map[string]int)
	for _, group := range groups {
		messageThreadID[group.Name] = group.ID
	}

	return &TelegramBot{
		bot:             bot,
		chatID:          defaultChatID,
		messageThreadID: messageThreadID,
	}, nil
}

// SendMessage envia uma mensagem de texto para o chat especificado
func (b *TelegramBot) SendMessage(ctx context.Context, text string, cameraName string) error {
	messageThreadID := b.getMessageThreadID(cameraName)
	_, err := b.bot.SendMessage(ctx, &tgbotapi.SendMessageParams{
		ChatID:          b.chatID,
		MessageThreadID: messageThreadID,
		Text:            text,
	})
	if err != nil {
		return fmt.Errorf("erro ao enviar mensagem: %w", err)
	}
	return nil
}

// SendPhoto envia uma foto para o chat especificado
func (b *TelegramBot) SendPhoto(ctx context.Context, photoBytes []byte, caption string, cameraName string) error {
	messageThreadID := b.getMessageThreadID(cameraName)
	photo := &models.InputMediaPhoto{
		Media:           "attach://" + uuid.New().String(),
		MediaAttachment: bytes.NewReader(photoBytes),
		Caption:         caption,
	}

	medias := []models.InputMedia{
		photo,
	}

	_, err := b.bot.SendMediaGroup(ctx, &tgbotapi.SendMediaGroupParams{
		ChatID:          b.chatID,
		MessageThreadID: messageThreadID,
		Media:           medias,
	})
	if err != nil {
		return fmt.Errorf("erro ao enviar foto: %w", err)
	}
	return nil
}

// SendVideo envia um vídeo para o chat especificado
func (b *TelegramBot) SendVideo(ctx context.Context, videoBytes []byte, caption string, cameraName string) error {
	messageThreadID := b.getMessageThreadID(cameraName)
	video := &models.InputMediaVideo{
		Media:           "attach://" + uuid.New().String(),
		MediaAttachment: bytes.NewReader(videoBytes),
		Caption:         caption,
	}

	medias := []models.InputMedia{
		video,
	}

	_, err := b.bot.SendMediaGroup(ctx, &tgbotapi.SendMediaGroupParams{
		ChatID:          b.chatID,
		MessageThreadID: messageThreadID,
		Media:           medias,
	})
	if err != nil {
		return fmt.Errorf("erro ao enviar vídeo: %w", err)
	}
	return nil
}

// getChatID retorna o ID do chat para uma câmera específica
func (b *TelegramBot) getMessageThreadID(cameraName string) int {
	if messageThreadID, ok := b.messageThreadID[cameraName]; ok {
		return messageThreadID
	}
	// Se não encontrar um grupo específico para a câmera, usar o grupo padrão
	log.Printf("Aviso: Grupo não encontrado para câmera '%s', usando grupo padrão", cameraName)
	return b.messageThreadID[cameraName]
}
