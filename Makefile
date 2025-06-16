COVEROUT ?= coverage.html

.PHONY: test build

test:
	go test -v ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o ${COVEROUT}

build:
	go build -o caesura main.go

run: build
	./caesura
