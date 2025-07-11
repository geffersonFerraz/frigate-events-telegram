package config

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Group represents a Telegram group with its name and ID
type Group struct {
	Name string
	ID   int64
}

// Config struct to store application configuration
// The 'mapstructure' tags now correspond to the keys in YAML
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

// LoadConfig loads configuration from a config.yaml file.
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Configure Viper to read the config.yaml file
	v.AddConfigPath(".")      // Search in current directory
	v.SetConfigName("config") // File name (without extension)
	v.SetConfigType("yaml")   // File type

	// Try to read the configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// File not found, return clear error
			log.Println("Error: Configuration file 'config.yaml' not found.")
			return nil, errors.New("config.yaml file not found")
		} else {
			// Other error reading the file
			log.Printf("Error reading configuration file 'config.yaml': %v", err)
			return nil, err
		}
	}

	// Set default values (still useful if the key is missing in YAML)
	v.SetDefault("mqtt_broker", "tcp://localhost:1883")
	v.SetDefault("mqtt_topic", "frigate/events")
	v.SetDefault("frigate_url", "http://localhost:5000")
	v.SetDefault("redis_addr", "localhost:6379")
	v.SetDefault("redis_password", "")
	v.SetDefault("redis_db", 0)
	v.SetDefault("use_thread_ids", false)
	v.SetDefault("timezone_ajust", 0)
	v.SetDefault("check_telegram", false)

	// Deserialize the read configuration to the Config struct
	var cfg Config
	err := v.Unmarshal(&cfg)
	if err != nil {
		log.Printf("Error deserializing configuration: %v", err)
		return nil, err
	}

	// Process Telegram groups
	groups := v.GetStringSlice("groups")
	cfg.Groups = make([]Group, 0, len(groups))
	for _, groupStr := range groups {
		parts := strings.Split(groupStr, "|")
		if len(parts) != 2 {
			log.Printf("Warning: Invalid format for group: %s", groupStr)
			continue
		}
		var group Group
		group.Name = parts[0]
		var id int64
		if _, err := fmt.Sscanf(parts[1], "%d", &id); err != nil {
			log.Printf("Warning: Invalid ID for group %s: %s", group.Name, parts[1])
			continue
		}
		group.ID = id
		cfg.Groups = append(cfg.Groups, group)
	}

	// Minimum validation (important, as defaults can mask absence)
	if cfg.TelegramToken == "" {
		log.Println("Error: 'telegram_token' not defined in config.yaml")
		return nil, errors.New("'telegram_token' not defined in config.yaml")
	}
	if cfg.TelegramChatID == 0 {
		log.Println("Error: 'telegram_chat_id' not defined in config.yaml")
		return nil, errors.New("'telegram_chat_id' not defined in config.yaml")
	}
	// FrigateURL has a default, so it doesn't need to be fatal if missing in yaml

	log.Println("Configuration loaded from config.yaml")
	return &cfg, nil
}
