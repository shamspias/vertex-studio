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

func Ptr[T any](v T) *T { return &v }

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
	scriptPath := flag.String("script-file", "scripts/scripts.json", "Path to the JSON script file")
	_ = flag.String("config", "", "Legacy flag")
	flag.Parse()

	ctx := context.Background()
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config Error: %v", err)
	}

	aiService, err := ai.NewService(ctx, cfg)
	if err != nil {
		log.Fatalf("Service Error: %v", err)
	}

	script, err := loadScript(*scriptPath)
	if err != nil {
		log.Fatalf("❌ Script Load Error: %v", err)
	}

	fmt.Printf("\n🎬 --- BATCH GENERATION STARTING ---\n")
	fmt.Printf("📂 Script: %s\n", *scriptPath)
	fmt.Printf("📹 Total Segments: %d\n", len(script.Segments))

	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	// OPTIMIZATION: Reduced concurrency to 2 to prevent "High Load" errors.
	// It's actually faster because fewer requests fail and restart.
	maxConcurrent := 2
	sem := make(chan struct{}, maxConcurrent)

	for i, seg := range script.Segments {
		wg.Add(1)
		idx := i
		segment := seg

		go func() {
			defer wg.Done()
			sem <- struct{}{} // Wait for slot
			defer func() { <-sem }()

			jobID := fmt.Sprintf("Seg-%02d", idx+1)
			filename := filepath.Join(outputDir, fmt.Sprintf("segment_%02d.mp4", idx+1))

			// Skip if file already exists (Resume capability)
			if _, err := os.Stat(filename); err == nil {
				fmt.Printf("[%s] ⏭️  File exists, skipping.\n", jobID)
				return
			}

			vidConfig := &genai.GenerateVideosConfig{
				AspectRatio:      script.Global.AspectRatio,
				PersonGeneration: script.Global.PersonGeneration,
				GenerateAudio:    Ptr(script.Global.GenerateAudio),
				NegativePrompt:   script.Global.NegativePrompt,
				Resolution:       script.Global.Resolution,
				FPS:              Ptr(script.Global.FPS),
				DurationSeconds:  Ptr(segment.Duration),
			}

			video, err := aiService.GenerateVideo(ctx, jobID, script.Global.Model, segment.Prompt, vidConfig)
			if err != nil {
				log.Printf("[%s] ❌ FINAL FAIL: %v", jobID, err)
				return
			}

			if err := saveVideo(video, filename, jobID); err != nil {
				log.Printf("[%s] ❌ Save Error: %v", jobID, err)
			}
		}()
	}

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
		return os.WriteFile(path, video.VideoBytes, 0644)
	}
	if video.URI != "" {
		fmt.Printf("[%s] ☁️  Downloading...\n", jobID)
		cmd := exec.Command("gcloud", "storage", "cp", video.URI, path)
		return cmd.Run()
	}
	return fmt.Errorf("no content")
}
