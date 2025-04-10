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
	// Removido tgbot de propósito pois não é usado diretamente em main agora

	"github.com/geffersonFerraz/frigate-events-telegram/config" // Import relativo ao módulo go
	"github.com/geffersonFerraz/frigate-events-telegram/mqtt_handler"
	"github.com/geffersonFerraz/frigate-events-telegram/redis_handler"
	"github.com/geffersonFerraz/frigate-events-telegram/telegram_handler"
)

// FrigateEvent representa a estrutura básica de um evento do Frigate (pode precisar de mais campos)
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

// AppHandler contém as dependências necessárias para o handler MQTT
type AppHandler struct {
	tgBot      *telegram_handler.TelegramBot
	cfg        *config.Config
	httpClient *http.Client // Para buscar a imagem
	redis      *redis_handler.RedisHandler
}

// newAppHandler cria uma nova instância do AppHandler
func newAppHandler(bot *telegram_handler.TelegramBot, cfg *config.Config, redis *redis_handler.RedisHandler) *AppHandler {
	return &AppHandler{
		tgBot:      bot,
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second}, // Timeout de 10s para buscar imagem
		redis:      redis,
	}
}

// downloadVideo tenta baixar o vídeo do Frigate, com retry se necessário
func (h *AppHandler) downloadVideo(ctx context.Context, clipURL string, maxRetries int) ([]byte, error) {
	var videoBytes []byte
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("Tentativa %d de %d de baixar o clipe %s", attempt, maxRetries, clipURL)
			// Esperar um pouco antes de tentar novamente
			time.Sleep(2 * time.Second)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", clipURL, nil)
		if err != nil {
			lastErr = fmt.Errorf("erro ao criar request: %w", err)
			continue
		}

		resp, err := h.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("erro ao buscar clipe: %w", err)
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
			lastErr = fmt.Errorf("erro ao ler bytes do clipe: %w", err)
			continue
		}

		if len(videoBytes) > 0 {
			return videoBytes, nil
		}

		lastErr = fmt.Errorf("clipe vazio recebido")
	}

	return nil, fmt.Errorf("falha após %d tentativas: %v", maxRetries, lastErr)
}

// processVideoEvent processa o download e envio do vídeo em uma goroutine separada
func (h *AppHandler) processVideoEvent(ctx context.Context, event FrigateEvent, clipURL string) {
	// Criar um contexto com timeout para todo o processo
	videoCtx, videoCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer videoCancel()

	// Channel para receber o resultado do processamento
	resultChan := make(chan error, 1)

	go func() {
		// Tentar baixar o vídeo (com retry)
		videoBytes, err := h.downloadVideo(videoCtx, clipURL, 9) // 3 tentativas
		if err != nil {
			resultChan <- fmt.Errorf("falha ao baixar vídeo: %w", err)
			return
		}

		// Criar legenda para o vídeo
		caption := fmt.Sprintf("Frigate Event Clip: %s\nCamera: %s\nTimestamp: %s",
			event.After.Label,
			event.After.Camera,
			time.Unix(int64(event.After.StartTime), 0).Format(time.RFC1123))

		log.Printf("Tentando enviar clipe do evento %s (%d bytes) para o Telegram...", event.After.ID, len(videoBytes))

		// Enviar vídeo pelo Telegram
		if err := h.tgBot.SendVideo(videoCtx, videoBytes, caption, event.After.Camera); err != nil {
			resultChan <- fmt.Errorf("erro ao enviar vídeo: %w", err)
			return
		}

		resultChan <- nil
	}()

	// Aguardar o resultado ou timeout
	select {
	case err := <-resultChan:
		if err != nil {
			log.Printf("Erro no processamento do vídeo para evento %s: %v", event.After.ID, err)
		} else {
			log.Printf("Clipe do evento %s enviado para o Telegram com sucesso.", event.After.ID)
		}
	case <-videoCtx.Done():
		log.Printf("Timeout ao processar vídeo do evento %s: %v", event.After.ID, videoCtx.Err())
	}
}

// handleMQTTMessage é o método que processa as mensagens MQTT
func (h *AppHandler) handleMQTTMessage(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Recebido: %s do tópico: %s\n", msg.Payload(), msg.Topic())

	var event FrigateEvent
	if err := json.Unmarshal(msg.Payload(), &event); err != nil {
		log.Printf("Erro ao decodificar JSON do evento: %v", err)
		return
	}

	// Verificar se o evento já foi processado
	ctx := context.Background()
	processed, err := h.redis.IsEventProcessed(ctx, event.After.ID, event.Type)
	if err != nil {
		log.Printf("Erro ao verificar evento no Redis: %v", err)
		return
	}
	if processed {
		log.Printf("Evento %s (tipo: %s) já foi processado anteriormente, ignorando.", event.After.ID, event.Type)
		return
	}

	// Queremos enviar apenas para eventos novos ou atualizados que tenham snapshot
	if (event.Type == "new" || event.Type == "update") && event.After.HasSnapshot {
		log.Printf("Processando evento '%s' para camera '%s' (ID: %s)", event.After.Label, event.After.Camera, event.After.ID)

		// Construir URL do snapshot
		snapshotURL := fmt.Sprintf("%s/api/events/%s/snapshot.jpg", strings.TrimSuffix(h.cfg.FrigateURL, "/"), event.After.ID)

		// Baixar a imagem
		req, err := http.NewRequestWithContext(context.Background(), "GET", snapshotURL, nil)
		if err != nil {
			log.Printf("Erro ao criar request para snapshot %s: %v", snapshotURL, err)
			return
		}

		resp, err := h.httpClient.Do(req)
		if err != nil {
			log.Printf("Erro ao buscar snapshot %s: %v", snapshotURL, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Erro ao buscar snapshot %s: Status %d", snapshotURL, resp.StatusCode)
			bodyBytes, _ := io.ReadAll(resp.Body)
			log.Printf("Corpo da resposta: %s", string(bodyBytes))
			return
		}

		imgBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Erro ao ler bytes do snapshot %s: %v", snapshotURL, err)
			return
		}

		// Criar legenda para a foto
		caption := fmt.Sprintf("Frigate Event: %s\nCamera: %s\nTimestamp: %s",
			event.After.Label,
			event.After.Camera,
			time.Unix(int64(event.After.StartTime), 0).Format(time.RFC1123))

		// Enviar foto pelo Telegram
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.tgBot.SendPhoto(ctx, imgBytes, caption, event.After.Camera); err != nil {
			log.Printf("Erro ao enviar foto para o Telegram: %v", err)
		}
		log.Printf("Foto do evento %s enviada para o Telegram.", event.After.ID)

		// Marcar evento como processado após enviar a foto
		if err := h.redis.MarkEventAsProcessed(ctx, event.After.ID, event.Type); err != nil {
			log.Printf("Erro ao marcar evento como processado no Redis: %v", err)
		}

	} else if event.Type == "end" && event.After.HasClip {
		log.Printf("Processando fim de evento '%s' para camera '%s' (ID: %s) - Enviando clipe.", event.After.Label, event.After.Camera, event.After.ID)

		// Construir URL do clipe
		clipURL := fmt.Sprintf("%s/api/events/%s/clip.mp4", strings.TrimSuffix(h.cfg.FrigateURL, "/"), event.After.ID)

		// Processar o vídeo em uma goroutine separada
		h.processVideoEvent(context.Background(), event, clipURL)

		// Marcar evento como processado após iniciar o processamento do vídeo
		if err := h.redis.MarkEventAsProcessed(ctx, event.After.ID, event.Type); err != nil {
			log.Printf("Erro ao marcar evento como processado no Redis: %v", err)
		}

	} else {
		// log.Printf("Evento ignorado (Tipo: %s, Snapshot: %t, Clip: %t)", event.Type, event.After.HasSnapshot, event.After.HasClip)
	}
}

func main() {
	fmt.Println("Iniciando Frigate Events Telegram...")

	// Carregar configuração
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Erro ao carregar configuração: %v", err)
	}

	// Inicializar Redis
	redis, err := redis_handler.NewRedisHandler(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Erro ao inicializar Redis: %v", err)
	}
	defer redis.Close()

	// Inicializar bot do Telegram
	tgBot, err := telegram_handler.NewBot(cfg.TelegramToken, cfg.TelegramChatID, cfg.Groups)
	if err != nil {
		log.Fatalf("Erro ao inicializar bot do Telegram: %v", err)
	}
	// TODO: Armazenar a instância do bot para uso posterior no messageHandler -> FEITO via AppHandler

	// Inicializar cliente MQTT
	// Usar um ClientID único se múltiplas instâncias forem rodar
	mqttClient, err := mqtt_handler.NewClient(cfg.MQTTBroker, "frigate-event-listener", cfg.MQTTUser, cfg.MQTTPassword)
	if err != nil {
		log.Fatalf("Erro ao inicializar cliente MQTT: %v", err)
	}

	// Criar o handler da aplicação
	appHandler := newAppHandler(tgBot, cfg, redis)

	// Inscrever no tópico de eventos do Frigate usando o método do handler
	if err := mqttClient.Subscribe(cfg.MQTTTopic, 1, appHandler.handleMQTTMessage); err != nil { // QoS 1: Pelo menos uma vez
		log.Fatalf("Erro ao inscrever no tópico MQTT: %v", err)
	}

	// Enviar mensagem de inicialização para o Telegram
	startupMessage := "✅ Bot Frigate Events Telegram inicializado com sucesso! Aguardando eventos..."
	if err := tgBot.SendMessage(context.Background(), startupMessage, "General"); err != nil {
		log.Printf("Aviso: Falha ao enviar mensagem de inicialização para o Telegram: %v", err)
	}

	fmt.Println("Aplicação pronta. Aguardando eventos MQTT...")

	// Esperar por sinal de interrupção para finalizar
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Finalizando...")
	mqttClient.Disconnect()
}
