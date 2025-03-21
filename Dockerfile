FROM golang:1.20-alpine AS builder

# Set working directory
WORKDIR /app

# Install required dependencies for CGO
RUN apk add --no-cache gcc musl-dev

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o grind_review_bot ./cmd

# Create minimal production image
FROM alpine:latest

# Install required runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from build stage
COPY --from=builder /app/grind_review_bot /app/grind_review_bot

# Copy configuration
COPY --from=builder /app/config/config.yaml /app/config/config.yaml

# Set working directory
WORKDIR /app

# Create volume for persistent storage
VOLUME ["/app/data"]

# Command to run
ENTRYPOINT ["/app/grind_review_bot"]