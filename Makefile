BINARY_NAME=studio
OUTPUT_DIR=output
SCRIPTS_DIR=scripts
SCRIPT_FILE=$(SCRIPTS_DIR)/scripts.json

.PHONY: all build run stitch clean init

all: build

# Create scripts directory and default script if not exists
init:
	@mkdir -p $(SCRIPTS_DIR)
	@if [ ! -f $(SCRIPT_FILE) ]; then \
		echo '{ \
  "global_settings": { \
    "model": "veo-3.1-generate-001", \
    "aspect_ratio": "16:9", \
    "resolution": "1080p", \
    "person_generation": "allow_adult", \
    "generate_audio": true, \
    "negative_prompt": "bad quality, distortion", \
    "fps": 24 \
  }, \
  "segments": [ \
    { \
      "duration": 5, \
      "prompt": "A cinematic shot of a futuristic city" \
    } \
  ] \
}' > $(SCRIPT_FILE); \
		echo "Created default script at $(SCRIPT_FILE)"; \
	else \
		echo "Script file already exists."; \
	fi

build: init
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) cmd/api/main.go

# Run the generation process
run: build
	@mkdir -p $(OUTPUT_DIR)
	./bin/$(BINARY_NAME) -script-file $(SCRIPT_FILE)

# Stitch generated segments into a final movie
stitch:
	@echo "Checking for segments in $(OUTPUT_DIR)..."
	@if ls $(OUTPUT_DIR)/segment_*.mp4 1> /dev/null 2>&1; then \
		echo "Stitching video segments..." && \
		rm -f ffmpeg_concat_list.txt && \
		for f in $(OUTPUT_DIR)/segment_*.mp4; do \
			echo "file '$$f'" >> ffmpeg_concat_list.txt; \
		done && \
		ffmpeg -y -f concat -safe 0 -i ffmpeg_concat_list.txt -c copy $(OUTPUT_DIR)/final_movie.mp4 && \
		echo "✅ Movie saved to $(OUTPUT_DIR)/final_movie.mp4"; \
	else \
		echo "❌ No segments found to stitch. Run 'make run' first."; \
	fi

clean:
	rm -rf bin
	rm -rf $(OUTPUT_DIR)/*
	rm -f ffmpeg_concat_list.txt
