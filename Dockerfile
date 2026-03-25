# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install dependencies for generating code and building
RUN apk add --no-cache git make nodejs npm
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN go install github.com/a-h/templ/cmd/templ@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generate swagger docs and templ templates
RUN swag init --generalInfo cmd/api/main.go --parseDependency --parseInternal --output docs/swagger --quiet
RUN templ generate

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o api_bin ./cmd/api

# Final stage
FROM alpine:latest

WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/api_bin .

# Copy static files
COPY static/ ./static/

# App will listen on port specified by APP_PORT env var (usually 8080)
EXPOSE 8080

CMD ["./api_bin"]