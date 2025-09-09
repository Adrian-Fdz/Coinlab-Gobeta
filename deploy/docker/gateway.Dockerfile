# Etapa de build
FROM golang:1.23 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/gateway/ ./cmd/gateway/

RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway

# Imagen final (ligera)
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/gateway .
EXPOSE 8080
ENTRYPOINT ["/app/gateway"]
