package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"google.golang.org/genai"

	"auto-cinema-studio/internal/config"
)

type Service struct {
	g         *genkit.Genkit
	veoClient *genai.Client
}

func NewService(ctx context.Context, cfg *config.Config) (*Service, error) {
	g := genkit.Init(ctx, genkit.WithPlugins(
		&googlegenai.GoogleAI{APIKey: cfg.GoogleGenAIKey},
		&googlegenai.VertexAI{ProjectID: cfg.ProjectID, Location: cfg.Location},
	))

	veoClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  cfg.ProjectID,
		Location: cfg.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Veo client: %w", err)
	}

	return &Service{g: g, veoClient: veoClient}, nil
}

func (s *Service) GenerateAnswer(ctx context.Context, modelName string, prompt string) (string, error) {
	return genkit.GenerateText(ctx, s.g,
		ai.WithModelName(modelName),
		ai.WithPrompt(prompt),
	)
}

// GenerateVideo with Smart Retry Logic
func (s *Service) GenerateVideo(ctx context.Context, jobID string, modelName string, prompt string, cfg *genai.GenerateVideosConfig) (*genai.Video, error) {
	cleanName := strings.TrimPrefix(modelName, "vertexai/")
	maxRetries := 3

	// Outer loop handles restarts if the server gets overloaded
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("[%s] 🔄 Retry attempt %d/%d...\n", jobID, attempt, maxRetries)
		}

		fmt.Printf("[%s] 🚀 Sending request...\n", jobID)
		op, err := s.veoClient.Models.GenerateVideos(ctx, cleanName, prompt, nil, cfg)
		if err != nil {
			// If immediate fail (e.g. quota), wait and retry
			log.Printf("[%s] ⚠️ Start failed: %v. Waiting...", jobID, err)
			time.Sleep(10 * time.Second)
			continue
		}
		fmt.Printf("[%s] ✅ Started: %s\n", jobID, op.Name)

		// Inner loop handles polling
		ticker := time.NewTicker(10 * time.Second)

		// Use a label to break out of the inner loop and continue the outer loop
	PollLoop:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return nil, ctx.Err()
			case <-ticker.C:
				op, err = s.veoClient.Operations.GetVideosOperation(ctx, op, nil)
				if err != nil {
					log.Printf("[%s] ⚠️ Poll check failed (network?): %v", jobID, err)
					continue
				}

				if op.Done {
					ticker.Stop()

					// Case 1: Failure
					if op.Error != nil {
						errMsg := fmt.Sprintf("%v", op.Error)
						// Check for overload/resource exhausted errors (Code 8)
						if isOverloadError(op.Error) {
							log.Printf("[%s] ⚠️ Server Overloaded. backing off...", jobID)
							time.Sleep(time.Duration(attempt*15) * time.Second) // Exponential backoff
							break PollLoop                                      // Break inner loop, triggering outer loop to RESTART generation
						}
						// Hard failure (e.g., safety filter), do not retry
						return nil, fmt.Errorf("generation error: %s", errMsg)
					}

					// Case 2: Success
					if op.Response != nil && len(op.Response.GeneratedVideos) > 0 {
						video := op.Response.GeneratedVideos[0].Video
						if video == nil {
							return nil, fmt.Errorf("video object is nil")
						}
						fmt.Printf("[%s] 🎉 Complete!\n", jobID)
						return video, nil
					}
					return nil, fmt.Errorf("no video found in response")
				}
				// Still running...
			}
		}
	}

	return nil, fmt.Errorf("failed after %d attempts", maxRetries)
}

// Helper to detect if error is temporary (Code 8 = Resource Exhausted)
func isOverloadError(errMap map[string]any) bool {
	// Check code
	if code, ok := errMap["code"].(float64); ok && int(code) == 8 {
		return true
	}
	if code, ok := errMap["code"].(int); ok && code == 8 {
		return true
	}
	// Check message text just in case
	if msg, ok := errMap["message"].(string); ok {
		msg = strings.ToLower(msg)
		return strings.Contains(msg, "high load") || strings.Contains(msg, "quota") || strings.Contains(msg, "try again")
	}
	return false
}
