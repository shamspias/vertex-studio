package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"vertex-studio/internal/ai"
	"vertex-studio/internal/config"
	"vertex-studio/internal/media"

	"github.com/joho/godotenv"
)

func main() {
	// 1. Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found. Ensure GOOGLE_CLOUD_PROJECT is set in environment.")
	}

	// 2. Parse Flags for Config File
	configPath := flag.String("config", "config/prompt.config", "Path to the JSON prompt config file")
	flag.Parse()

	// 3. Setup Output Directory
	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "output"
	}
	os.MkdirAll(outputDir, 0755)

	// 4. Init AI Client
	ctx := context.Background()
	aiClient, err := ai.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to init AI client: %v", err)
	}

	// 5. Interactive Menu
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("\n=== Vertex AI Studio (Config: %s) ===\n", *configPath)
		fmt.Println("1. Generate Movie (Veo 3.1)")
		fmt.Println("2. Exit")
		fmt.Print("Select option: ")

		if !scanner.Scan() {
			break
		}
		choice := scanner.Text()

		switch choice {
		case "1":
			runMoviePipeline(ctx, aiClient, *configPath, outputDir)
		case "2":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid choice")
		}
	}
}

func runMoviePipeline(ctx context.Context, client *ai.Client, configPath, outputDir string) {
	// Load Config
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("Error loading config file: %v", err)
		return
	}

	var generatedSegments []string
	var lastFramePath string

	for i, segment := range cfg.Segments {
		fmt.Printf("\n--- Processing Segment %d/%d ---\n", i+1, len(cfg.Segments))

		fullPrompt := fmt.Sprintf("%s. %s", segment.Prompt, cfg.GlobalSettings.NegativePrompt)
		filename := filepath.Join(outputDir, fmt.Sprintf("segment_%d.mp4", i+1))

		// Call AI
		err := client.GenerateVideo(ctx, fullPrompt, lastFramePath, filename)
		if err != nil {
			log.Printf("Failed to generate segment %d: %v", i+1, err)
			break
		}

		generatedSegments = append(generatedSegments, filename)
		fmt.Printf("   -> Saved: %s\n", filename)

		// Extract frame for next segment (Continuity)
		if i < len(cfg.Segments)-1 {
			fmt.Println("   -> Extracting frame for continuity...")
			framePath, err := media.ExtractLastFrame(filename)
			if err != nil {
				log.Printf("Warning: Could not extract frame: %v", err)
				lastFramePath = ""
			} else {
				lastFramePath = framePath
			}
		}
	}

	// Stitch
	if len(generatedSegments) > 0 {
		fmt.Println("\n--- Stitching Final Movie ---")
		finalOutput := filepath.Join(outputDir, "final_movie.mp4")
		if err := media.StitchVideos(generatedSegments, finalOutput); err != nil {
			log.Printf("Error stitching: %v", err)
		} else {
			fmt.Printf("SUCCESS! Movie ready at: %s\n", finalOutput)
		}
	} else {
		fmt.Println("No videos were generated.")
	}
}
