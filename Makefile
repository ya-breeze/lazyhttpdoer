run:
	mkdir -p flows
	go run internal/main.go

lint:
	@golangci-lint run

.PHONY: run lint
