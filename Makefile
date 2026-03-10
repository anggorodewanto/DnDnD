.PHONY: build test cover run docker-build clean

build:
	go build -o bin/dndnd ./cmd/dndnd/

test:
	go test ./... -v

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

run:
	go run ./cmd/dndnd/

docker-build:
	docker build -t dndnd .

clean:
	rm -rf bin/ coverage.out
