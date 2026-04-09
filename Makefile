BIN := scout
BUILD_TARGET := .
INSTALL_DIR ?= $(HOME)/.local/bin

build:
	go build -o $(BIN) $(BUILD_TARGET)

install: build
	mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN) $(INSTALL_DIR)/$(BIN)

.PHONY: build install
