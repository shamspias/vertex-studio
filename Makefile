BINARY_NAME=studio
OUTPUT_DIR=output
CONFIG_DIR=config
CONFIG_FILE=$(CONFIG_DIR)/prompt.config

.PHONY: all build run clean init

all: build

# Create config directory and default config if not exists
init:
	@mkdir -p $(CONFIG_DIR)
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo '{"global_settings":{"aspect_ratio":"16:9","negative_prompt":"bad quality"},"segments":[{"duration":5,"prompt":"A cinematic shot of a futuristic city"}]}' > $(CONFIG_FILE); \
		echo "Created default config at $(CONFIG_FILE)"; \
	else \
		echo "Config file already exists."; \
	fi

build: init
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) cmd/studio/main.go

run: build
	@mkdir -p $(OUTPUT_DIR)
	./bin/$(BINARY_NAME) -config $(CONFIG_FILE)

clean:
	rm -rf bin
	rm -rf $(OUTPUT_DIR)/*
	rm -f ffmpeg_concat_list.txt