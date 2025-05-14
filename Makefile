.PHONY: build-all docker-all

build-all:
	go build -o bin/cacher ./cmd/cacher
	go build -o bin/fetcher ./cmd/fetcher
	go build -o bin/signal_gen ./cmd/signal_gen

docker-all:
	docker build -t signal_gen -f ./cmd/signal_gen/Dockerfile .
	docker build -t fetcher -f ./cmd/fetcher/Dockerfile .
	docker build -t cacher -f ./cmd/cacher/Dockerfile .