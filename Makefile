GO ?= go
BIN := hbtop

.PHONY: build run install clean

build:
	$(GO) build -o $(BIN) ./cmd/hbtop

run: build
	./$(BIN)

install: build
	install -Dm755 $(BIN) $(HOME)/.local/bin/$(BIN)

clean:
	rm -f $(BIN)
