# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w \
      -X github.com/HMetcalfeW/cartographer/cmd/version.Version=${VERSION} \
      -X github.com/HMetcalfeW/cartographer/cmd/version.Commit=${COMMIT} \
      -X github.com/HMetcalfeW/cartographer/cmd/version.Date=${DATE}" \
    -o /cartographer .

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates helm
RUN mkdir /output
COPY --from=builder /cartographer /usr/local/bin/cartographer
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]