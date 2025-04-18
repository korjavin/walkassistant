FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /walkassistant ./backend

# Use a minimal alpine image for the final container
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests (needed for OSRM API)
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /walkassistant .

# Copy frontend files
COPY frontend/ ./frontend/

# Create data directory
RUN mkdir -p data

# Expose port 8080
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["./walkassistant"]
