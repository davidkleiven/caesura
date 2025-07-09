COVEROUT ?= coverage.html

.PHONY: unittest uitest build

unittest:
	go list ./... | grep -v web_test | xargs go test -v -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o ${COVEROUT}

uitest:
	go test -v ./web_test

test: uitest unittest

build:
	go build -o caesura main.go

run: build
	./caesura
