.PHONY: docker-compose test

docker-compose:
	sudo docker compose build
	sudo docker compose up

test:
	go test -v ./cmd/...
