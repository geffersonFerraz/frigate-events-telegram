# Build stage
FROM golang:1.25.3-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o frigate-events-telegram -ldflags="-s -w"

# Final stage
FROM ubuntu:22.04

WORKDIR /app

# Install minimal required dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy the compiled binary from the build stage
COPY --from=builder /app/frigate-events .
COPY --from=builder /app/config.yaml.example .

# Create non-root user
RUN useradd -r -u 1000 appuser && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set environment variables
ENV TZ=America/Sao_Paulo

# Run the binary
CMD ["./frigate-events"]