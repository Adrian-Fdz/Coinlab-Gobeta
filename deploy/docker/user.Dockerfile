# Etapa de build
FROM golang:1.22 AS builder
WORKDIR /app

# Copiamos go.mod y go.sum primero (mejor caché)
COPY go.mod go.sum ./
RUN go mod download

# Copiamos el código del servicio
COPY cmd/user/ ./cmd/user/

# Compilamos el binario
RUN CGO_ENABLED=0 GOOS=linux go build -o user-service ./cmd/user

# Imagen final (ligera)
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/user-service .
EXPOSE 8081
ENTRYPOINT ["/app/user-service"]
