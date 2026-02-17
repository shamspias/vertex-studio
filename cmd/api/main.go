package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"google.golang.org/genai"

	"auto-cinema-studio/internal/ai"
	"auto-cinema-studio/internal/config"
)

// Ptr helper
func Ptr[T any](v T) *T { return &v }

// JSON Structures
type ScriptFile struct {
	Global   GlobalSettings `json:"global_settings"`
	Segments []Segment      `json:"segments"`
}

type GlobalSettings struct {
	Model            string `json:"model"`
	AspectRatio      string `json:"aspect_ratio"`
	Resolution       string `json:"resolution"`
	PersonGeneration string `json:"person_generation"`
	GenerateAudio    bool   `json:"generate_audio"`
	NegativePrompt   string `json:"negative_prompt"`
	FPS              int32  `json:"fps"`
}

type Segment struct {
	Duration int32  `json:"duration"`
	Prompt   string `json:"prompt"`
}

func main() {
	// 1. Flags
	scriptPath := flag.String("script-file", "scripts/scripts.json", "Path to the JSON script file")
	_ = flag.String("config", "", "Legacy config flag") // Prevent make errors
	flag.Parse()

	// 2. Setup
	ctx := context.Background()
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config Error: %v", err)
	}

	aiService, err := ai.NewService(ctx, cfg)
	if err != nil {
		log.Fatalf("Service Error: %v", err)
	}

	// 3. Load Script
	script, err := loadScript(*scriptPath)
	if err != nil {
		log.Fatalf("❌ Failed to load script: %v", err)
	}

	fmt.Printf("\n🎬 --- BATCH VIDEO GENERATION STARTING ---\n")
	fmt.Printf("📂 Script: %s\n", *scriptPath)
	fmt.Printf("📹 Segments to generate: %d\n", len(script.Segments))
	fmt.Printf("⚙️  Global Model: %s\n", script.Global.Model)

	// Create output dir
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal(err)
	}

	// 4. Parallel Generation
	var wg sync.WaitGroup

	// Create a semaphore to limit concurrency if needed (optional, prevents hitting rate limits)
	// Example: Allow 4 concurrent requests. Change to len(script.Segments) for "all at once"
	maxConcurrent := 4
	sem := make(chan struct{}, maxConcurrent)

	for i, seg := range script.Segments {
		wg.Add(1)

		// Capture loop variables
		idx := i
		segment := seg

		go func() {
			defer wg.Done()

			// Acquire token
			sem <- struct{}{}
			defer func() { <-sem }() // Release token

			jobID := fmt.Sprintf("Seg-%02d", idx+1)
			filename := filepath.Join(outputDir, fmt.Sprintf("segment_%02d.mp4", idx+1))

			// Construct Config (Merge Global + Segment)
			vidConfig := &genai.GenerateVideosConfig{
				AspectRatio:      script.Global.AspectRatio,
				PersonGeneration: script.Global.PersonGeneration,
				GenerateAudio:    Ptr(script.Global.GenerateAudio),
				NegativePrompt:   script.Global.NegativePrompt,
				Resolution:       script.Global.Resolution, // e.g. "1080p"
				FPS:              Ptr(script.Global.FPS),
				DurationSeconds:  Ptr(segment.Duration),
			}

			// Generate
			video, err := aiService.GenerateVideo(ctx, jobID, script.Global.Model, segment.Prompt, vidConfig)
			if err != nil {
				log.Printf("[%s] ❌ Error: %v", jobID, err)
				return
			}

			// Save
			if err := saveVideo(video, filename, jobID); err != nil {
				log.Printf("[%s] ❌ Save Error: %v", jobID, err)
			}
		}()
	}

	// Wait for all
	wg.Wait()
	fmt.Println("\n🏁 --- ALL JOBS COMPLETED ---")
}

func loadScript(path string) (*ScriptFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var script ScriptFile
	if err := json.Unmarshal(bytes, &script); err != nil {
		return nil, err
	}

	// Defaults if missing in JSON
	if script.Global.Model == "" {
		script.Global.Model = "veo-2.0-generate-001"
	}
	if script.Global.FPS == 0 {
		script.Global.FPS = 24
	}
	if script.Global.PersonGeneration == "" {
		script.Global.PersonGeneration = "allow_adult"
	}

	return &script, nil
}

func saveVideo(video *genai.Video, path string, jobID string) error {
	if len(video.VideoBytes) > 0 {
		if err := os.WriteFile(path, video.VideoBytes, 0644); err != nil {
			return err
		}
		fmt.Printf("[%s] 💾 Saved to disk: %s\n", jobID, path)
		return nil
	}

	if video.URI != "" {
		fmt.Printf("[%s] ☁️  Downloading from GCS...\n", jobID)
		cmd := exec.Command("gcloud", "storage", "cp", video.URI, path)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		fmt.Printf("[%s] 💾 Downloaded to: %s\n", jobID, path)
		return nil
	}

	return fmt.Errorf("no content")
}
