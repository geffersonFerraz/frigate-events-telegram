# Use Go 1.24 bookworm as base image
FROM golang:1.24-bookworm AS base

# Move to working directory /build
WORKDIR /build

# Copy the go.mod and go.sum files to the /build directory
COPY go.mod go.sum ./

# Install dependencies
RUN go mod download

# Copy the entire source code into the container
COPY frigate-events-telegram .
COPY config.yaml.example .

# Document the port that may need to be published
USER 1000

# Start the application
CMD ["/frigate-events-telegram"]