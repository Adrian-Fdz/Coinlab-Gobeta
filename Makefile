build-user:
	go build -o bin/user ./cmd/user

build-gateway:
	go build -o bin/gateway ./cmd/gateway

build-keys:
	go build -o bin/keys ./cmd/keys

docker-user:
	docker build -f deploy/docker/user.Dockerfile -t cityletterbox/user .

docker-gateway:
	docker build -f deploy/docker/gateway.Dockerfile -t cityletterbox/gateway .

docker-keys:
	docker build -f deploy/docker/keys.Dockerfile -t cityletterbox/keys .

up:
	cd deploy && docker compose up --build

down:
	cd deploy && docker compose down -v

logs:
	cd deploy && docker compose logs -f

ps:
	cd deploy && docker compose ps
