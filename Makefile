build-user:
	go build -o bin/user ./cmd/user

build-gateway:
	go build -o bin/gateway ./cmd/gateway

docker-user:
	docker build -f deploy/docker/user.Dockerfile -t cityletterbox/user .

docker-gateway:
	docker build -f deploy/docker/gateway.Dockerfile -t cityletterbox/gateway .
