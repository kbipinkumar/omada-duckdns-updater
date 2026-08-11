# Build Stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY *.go ./

# Accept VERSION build arg
ARG VERSION=dev

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o omada-duckdns-updater

# Run Stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/omada-duckdns-updater .

# Expose the Web UI port
EXPOSE 5381

# Command to run the executable
CMD ["./omada-duckdns-updater"]
