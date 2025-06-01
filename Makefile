APP_NAME=golden-gate
MAIN=./cmd/main.go
BINARY=./golden-gate

.PHONY: all build run generate clean fmt docker-build docker-run

all: build

build:
	go build -o $(BINARY) $(MAIN)

generate:
	~/go/bin/templ generate ./internal/dashboard/views/

run: generate build
	./golden-gate

docker-build:
	docker build -t $(APP_NAME) .

docker-run:
	docker run -p 8080:8080 -v $(PWD)/configs:/app/configs $(APP_NAME)

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY) 