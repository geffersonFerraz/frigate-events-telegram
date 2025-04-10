# Estágio de build
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copiar arquivos de dependências
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fonte
COPY . .

# Compilar o binário
RUN CGO_ENABLED=0 GOOS=linux go build -o frigate-events-telegram -ldflags="-s -w"

# Estágio final
FROM ubuntu:22.04

WORKDIR /app

# Instalar dependências mínimas necessárias
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copiar o binário compilado do estágio de build
COPY --from=builder /app/frigate-events-telegram .
COPY --from=builder /app/config.yaml.example .

# Criar usuário não-root
RUN useradd -r -u 1000 appuser && \
    chown -R appuser:appuser /app

# Mudar para o usuário não-root
USER appuser

# Definir variáveis de ambiente
ENV TZ=America/Sao_Paulo

# Executar o binário
CMD ["./frigate-events-telegram"]