.PHONY: all test changelog

all: test stmps changelog

VERSION != git describe --tags HEAD

stmps:
	go build -ldflags="-X main.Version=$(VERSION)" -o stmps .

changelog:
	git cliff -o CHANGELOG.md

test:
	go test ./...
	markdownlint README.md
	golangci-lint run
