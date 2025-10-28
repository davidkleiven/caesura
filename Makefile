COVEROUT ?= coverage.html

.PHONY: unittest uitest build css

unittest:
	go list ./... | grep -v web_test | xargs go test -failfast -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o ${COVEROUT}

uitest:
	go test -failfast ./web_test

test: uitest unittest

build:
	go build -o caesura main.go
css:
	npx tailwindcss -i ./web/css/input.css -o web/css/output.css

run: build
	./caesura
