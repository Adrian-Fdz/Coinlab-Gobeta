# Etapa de build
FROM golang:1.23 AS builder
WORKDIR /app

# Copiamos go.mod y go.sum primero (mejor caché)
COPY go.mod go.sum ./
RUN go mod download

# Copiamos el código del servicio
COPY cmd/keys/ ./cmd/keys/

# Compilamos el binario
RUN CGO_ENABLED=0 GOOS=linux go build -o keys-service ./cmd/keys

# Imagen final (ligera)
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/keys-service .
EXPOSE 8083
ENTRYPOINT ["/app/keys-service"]
