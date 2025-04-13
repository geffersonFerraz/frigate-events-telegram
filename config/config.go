package config

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Group representa um grupo do Telegram com seu nome e ID
type Group struct {
	Name string
	ID   int64
}

// Config struct para armazenar as configurações da aplicação
// As tags 'mapstructure' agora correspondem às chaves no YAML
type Config struct {
	MQTTBroker     string  `mapstructure:"mqtt_broker"`
	MQTTUser       string  `mapstructure:"mqtt_user"`
	MQTTPassword   string  `mapstructure:"mqtt_password"`
	MQTTTopic      string  `mapstructure:"mqtt_topic"`
	TelegramToken  string  `mapstructure:"telegram_token"`
	TelegramChatID int64   `mapstructure:"telegram_chat_id"`
	UseThreadIDs   bool    `mapstructure:"use_thread_ids"`
	FrigateURL     string  `mapstructure:"frigate_url"`
	RedisAddr      string  `mapstructure:"redis_addr"`
	RedisPassword  string  `mapstructure:"redis_password"`
	RedisDB        int     `mapstructure:"redis_db"`
	TimezoneAjust  int     `mapstructure:"timezone_ajust"`
	Groups         []Group `mapstructure:"-"`
	CheckTelegram  bool    `mapstructure:"check_telegram"`
}

// LoadConfig carrega as configurações de um arquivo config.yaml.
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Configurar Viper para ler o arquivo config.yaml
	v.AddConfigPath(".")      // Procurar no diretório atual
	v.SetConfigName("config") // Nome do arquivo (sem extensão)
	v.SetConfigType("yaml")   // Tipo do arquivo

	// Tentar ler o arquivo de configuração
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Arquivo não encontrado, retornar erro claro
			log.Println("Erro: Arquivo de configuração 'config.yaml' não encontrado.")
			return nil, errors.New("arquivo config.yaml não encontrado")
		} else {
			// Outro erro ao ler o arquivo
			log.Printf("Erro ao ler arquivo de configuração 'config.yaml': %v", err)
			return nil, err
		}
	}

	// Definir valores padrão (ainda úteis caso a chave esteja ausente no YAML)
	v.SetDefault("mqtt_broker", "tcp://localhost:1883")
	v.SetDefault("mqtt_topic", "frigate/events")
	v.SetDefault("frigate_url", "http://localhost:5000")
	v.SetDefault("redis_addr", "localhost:6379")
	v.SetDefault("redis_password", "")
	v.SetDefault("redis_db", 0)
	v.SetDefault("use_thread_ids", false)
	v.SetDefault("timezone_ajust", 0)
	v.SetDefault("check_telegram", false)

	// Deserializar a configuração lida para a struct Config
	var cfg Config
	err := v.Unmarshal(&cfg)
	if err != nil {
		log.Printf("Erro ao deserializar configuração: %v", err)
		return nil, err
	}

	// Processar os grupos do Telegram
	groups := v.GetStringSlice("groups")
	cfg.Groups = make([]Group, 0, len(groups))
	for _, groupStr := range groups {
		parts := strings.Split(groupStr, "|")
		if len(parts) != 2 {
			log.Printf("Aviso: Formato inválido para grupo: %s", groupStr)
			continue
		}
		var group Group
		group.Name = parts[0]
		var id int64
		if _, err := fmt.Sscanf(parts[1], "%d", &id); err != nil {
			log.Printf("Aviso: ID inválido para grupo %s: %s", group.Name, parts[1])
			continue
		}
		group.ID = id
		cfg.Groups = append(cfg.Groups, group)
	}

	// Validação mínima (importante, pois os padrões podem mascarar a ausência)
	if cfg.TelegramToken == "" {
		log.Println("Erro: 'telegram_token' não definido no config.yaml")
		return nil, errors.New("'telegram_token' não definido no config.yaml")
	}
	if cfg.TelegramChatID == 0 {
		log.Println("Erro: 'telegram_chat_id' não definido no config.yaml")
		return nil, errors.New("'telegram_chat_id' não definido no config.yaml")
	}
	// FrigateURL tem um padrão, então não precisa ser fatal se ausente no yaml

	log.Println("Configuração carregada de config.yaml")
	return &cfg, nil
}
