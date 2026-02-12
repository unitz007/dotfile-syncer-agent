# Multi-stage build for dotfile agent
FROM golang:1.22.3-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dotfile-agent .

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    git \
    bash \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 dotfile && \
    adduser -D -u 1000 -G dotfile dotfile

# Set working directory
WORKDIR /home/dotfile

# Copy binary from builder
COPY --from=builder /build/dotfile-agent /usr/local/bin/dotfile-agent

# Make binary executable
RUN chmod +x /usr/local/bin/dotfile-agent

# Create necessary directories
RUN mkdir -p /home/dotfile/.config/dotfile-agent && \
    mkdir -p /home/dotfile/dotfiles && \
    chown -R dotfile:dotfile /home/dotfile

# Switch to non-root user
USER dotfile

# Expose default port
EXPOSE 2000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:2000/sync || exit 1

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/dotfile-agent"]

# Default command (can be overridden)
CMD ["--port", "2000"]
