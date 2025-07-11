# Frigate Events Telegram

A Go application that integrates Frigate NVR with Telegram to send notifications and media when motion events are detected. This bot can send snapshots and video clips from Frigate cameras directly to Telegram chats or groups.

## Features

- **Real-time notifications**: Receives MQTT events from Frigate and sends instant notifications
- **Media sharing**: Automatically sends snapshots and video clips to Telegram
- **Multi-camera support**: Configure different cameras to send to different Telegram groups/threads
- **Command interface**: Built-in Telegram commands for system status, snapshots, and recordings
- **Duplicate prevention**: Uses Redis to prevent sending duplicate events
- **Docker support**: Easy deployment with Docker and Docker Compose

## Prerequisites

- Frigate NVR instance running and configured
- MQTT broker (usually included with Frigate)
- Redis server
- Telegram bot token (create one via [@BotFather](https://t.me/botfather))

## Installation

### Using Docker (Recommended)

1. Clone the repository:
```bash
git clone https://github.com/geffersonFerraz/frigate-events-telegram.git
cd frigate-events-telegram
```

2. Copy the example configuration:
```bash
cp config.yaml.example config.yaml
```

3. Edit `config.yaml` with your settings (see Configuration section below)

4. Run with Docker Compose:
```bash
docker-compose up -d
```

### Manual Installation

1. Ensure you have Go 1.24+ installed
2. Clone the repository and navigate to it
3. Copy and configure `config.yaml`
4. Run:
```bash
go mod download
go build -o frigate-events-telegram
./frigate-events-telegram
```

## Configuration

Edit `config.yaml` with your settings:

```yaml
# MQTT Configuration
mqtt_broker: tcp://localhost:1883
mqtt_user: ""
mqtt_password: ""
mqtt_topic: frigate/events

# Telegram Configuration
telegram_token: "YOUR_BOT_TOKEN_HERE"
telegram_chat_id: 123456789

# Frigate Configuration
frigate_url: "http://localhost:5000"

# Redis Configuration
redis_addr: "localhost:6379"
redis_password: ""
redis_db: 0

# Optional: Camera-specific groups (format: "CameraName|ThreadID")
groups:
  - General|1
  - FrontDoor|123
  - Backyard|456
```

### Configuration Options

- `mqtt_broker`: MQTT broker address
- `mqtt_topic`: Topic to subscribe to for Frigate events
- `telegram_token`: Your Telegram bot token
- `telegram_chat_id`: Default chat ID for notifications
- `frigate_url`: Frigate API base URL
- `redis_addr`: Redis server address
- `groups`: List of camera names and their corresponding Telegram thread IDs

## Usage

### Telegram Commands

Once the bot is running, you can use these commands in Telegram:

- `/status` - Shows system status and uptime
- `/snapshot` - Takes a snapshot from the current camera thread
- `/record [seconds]` - Creates a recording event (default 10 seconds)
- `/clean` - Clears Redis cache
- `/restart` - Restarts the bot
- `/help` - Shows available commands

### Event Types

The bot handles different types of Frigate events:

- **New events**: Sends snapshots when motion is first detected
- **Updated events**: Sends updated snapshots if the event changes
- **End events**: Sends video clips when the recording is complete

## Development

### Project Structure

```
frigate-events-telegram/
├── config/           # Configuration management
├── frigate/          # Frigate API client
├── mqtt_handler/     # MQTT client and handlers
├── redis_handler/    # Redis operations
├── telegram_handler/ # Telegram bot implementation
├── main.go          # Main application entry point
└── config.yaml      # Configuration file
```

### Building

```bash
# Build for current platform
go build -o frigate-events-telegram

# Build for Docker
docker build -t frigate-events-telegram .
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

If you encounter any issues or have questions, please open an issue on GitHub.