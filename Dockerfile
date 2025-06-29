# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Remove GOOS and GOARCH so buildx can set them appropriately
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /cartographer .

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /cartographer /usr/local/bin/cartographer
RUN chmod +x /usr/local/bin/cartographer
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/cartographer"]