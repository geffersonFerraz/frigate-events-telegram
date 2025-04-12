package telegram_handler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/geffersonFerraz/frigate-events-telegram/config"
	tgbotapi "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// CommandHandler gerencia os comandos recebidos pelo bot
type CommandHandler struct {
	bot             *tgbotapi.Bot
	chatIDs         []config.Group
	cameraThreadIDs map[int64]string
	useThreadIDs    bool
	handlers        map[string]CommandFunc
	mutex           sync.RWMutex
	stopCh          chan struct{}
}

// CommandFunc é o tipo de função que processa um comando
type CommandFunc func(ctx context.Context, args []string, cameraName string) string

// CommandConfig contém as configurações para o gerenciador de comandos
type CommandConfig struct {
	Bot          *tgbotapi.Bot
	ChatIDs      []config.Group
	UseThreadIDs bool
}

// NewCommandHandler cria um novo gerenciador de comandos
func NewCommandHandler(config CommandConfig) *CommandHandler {
	handler := &CommandHandler{
		bot:             config.Bot,
		chatIDs:         config.ChatIDs,
		cameraThreadIDs: make(map[int64]string),
		useThreadIDs:    config.UseThreadIDs,
		handlers:        make(map[string]CommandFunc),
		stopCh:          make(chan struct{}),
	}

	// Registrar comandos padrão
	handler.RegisterCommand("restart", handler.handleRestart)
	handler.RegisterCommand("snapshot", handler.handleSnapshot)
	handler.RegisterCommand("clean", handler.handleClean)
	handler.RegisterCommand("status", handler.handleStatus)
	handler.RegisterCommand("help", handler.handleHelp)

	return handler
}

// Start inicia o loop de processamento de comandos
func (h *CommandHandler) Start(ctx context.Context) {
	// Inicia o loop de processamento em background
	go h.processUpdates(ctx)
	log.Println("Gerenciador de comandos do Telegram iniciado")
}

// Stop interrompe o loop de processamento de comandos
func (h *CommandHandler) Stop() {
	close(h.stopCh)
	log.Println("Gerenciador de comandos do Telegram parado")
}

// EnableThreadIDs ativa ou desativa o uso de messageThreadID
func (h *CommandHandler) EnableThreadIDs(enabled bool) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.useThreadIDs = enabled
	log.Printf("Uso de messageThreadID %s", map[bool]string{true: "ativado", false: "desativado"}[enabled])
}

// RegisterCommand registra uma função para lidar com um comando específico
func (h *CommandHandler) RegisterCommand(command string, handler CommandFunc) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.handlers[command] = handler
	log.Printf("Comando registrado: %s", command)
}

// RegisterCameraThreadID associa um messageThreadID a uma câmera
func (h *CommandHandler) RegisterCameraThreadID(threadID int64, cameraName string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.cameraThreadIDs[threadID] = cameraName
	log.Printf("Thread ID %d associado à câmera %s", threadID, cameraName)
}

// processUpdates processa as atualizações recebidas do Telegram
func (h *CommandHandler) processUpdates(ctx context.Context) {
	for {
		select {
		case <-h.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// Verificar se há atualizações a cada segundo
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// ProcessUpdate processa uma atualização do Telegram
func (h *CommandHandler) ProcessUpdate(ctx context.Context, update *models.Update) {
	// Verificar se a atualização contém uma mensagem com comando
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	text := update.Message.Text
	if !strings.HasPrefix(text, "/") {
		return
	}

	// Extrair comando e argumentos
	parts := strings.Fields(text)
	command := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	// Identificar a câmera a partir do threadID, se disponível e habilitado
	var cameraName string
	if h.useThreadIDs && update.Message.MessageThreadID != 0 {
		h.mutex.RLock()
		camera, exists := h.cameraThreadIDs[int64(update.Message.MessageThreadID)]
		h.mutex.RUnlock()
		if exists {
			cameraName = camera
		}
	}

	// Executar o comando
	h.mutex.RLock()
	handler, exists := h.handlers[command]
	h.mutex.RUnlock()

	if !exists {
		// Comando não reconhecido
		h.respondToMessage(ctx, update.Message.Chat.ID, int64(update.Message.MessageThreadID),
			"Comando não reconhecido. Use /help para ver os comandos disponíveis.")
		return
	}

	// Executar o comando e obter a resposta
	response := handler(ctx, args, cameraName)
	h.respondToMessage(ctx, update.Message.Chat.ID, int64(update.Message.MessageThreadID), response)
}

// respondToMessage envia uma resposta para a mensagem original
func (h *CommandHandler) respondToMessage(ctx context.Context, chatID int64, threadID int64, text string) {
	params := &tgbotapi.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}

	if h.useThreadIDs && threadID != 0 {
		params.MessageThreadID = int(threadID)
	}

	_, err := h.bot.SendMessage(ctx, params)
	if err != nil {
		log.Printf("Erro ao enviar resposta: %v", err)
	}
}

// Implementação dos comandos

func (h *CommandHandler) handleRestart(ctx context.Context, args []string, cameraName string) string {
	// Aqui você implementaria a lógica para reiniciar o bot ou o sistema
	return "🔄 Comando de reinicialização recebido. Esta funcionalidade ainda não está implementada."
}

func (h *CommandHandler) handleSnapshot(ctx context.Context, args []string, cameraName string) string {
	// Verificar se uma câmera foi especificada
	targetCamera := cameraName
	if len(args) > 0 {
		targetCamera = args[0]
	}

	if targetCamera == "" {
		return "⚠️ Nenhuma câmera especificada. Use /snapshot [nome_da_camera] ou execute o comando em um tópico específico de câmera."
	}

	// Aqui você implementaria a lógica para capturar um snapshot da câmera
	return fmt.Sprintf("📸 Solicitação de snapshot para câmera '%s' recebida. Esta funcionalidade ainda não está implementada.", targetCamera)
}

func (h *CommandHandler) handleClean(ctx context.Context, args []string, cameraName string) string {
	// Aqui você implementaria a lógica para limpar dados temporários
	return "🧹 Comando de limpeza recebido. Esta funcionalidade ainda não está implementada."
}

func (h *CommandHandler) handleStatus(ctx context.Context, args []string, cameraName string) string {
	// Aqui você implementaria a lógica para verificar o status
	statusInfo := []string{
		"✅ Sistema em execução",
		"🕒 Tempo de atividade: [tempo desde a inicialização]",
		"🔄 Eventos processados: [número de eventos]",
		"📊 Uso de memória: [uso de memória]",
	}

	if cameraName != "" {
		statusInfo = append(statusInfo, fmt.Sprintf("📷 Câmera selecionada: %s", cameraName))
	}

	return strings.Join(statusInfo, "\n")
}

func (h *CommandHandler) handleHelp(ctx context.Context, args []string, cameraName string) string {
	// Lista de comandos disponíveis
	commands := []string{
		"🔄 /restart - Reinicia o bot",
		"📸 /snapshot [câmera] - Tira um snapshot da câmera especificada",
		"🧹 /clean - Limpa dados temporários",
		"ℹ️ /status - Mostra o status do sistema",
		"❓ /help - Mostra esta mensagem de ajuda",
	}

	return "📋 *Comandos disponíveis:*\n" + strings.Join(commands, "\n")
}
