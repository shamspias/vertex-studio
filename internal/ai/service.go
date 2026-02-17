package ai

import (
	"context"
	"fmt"
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

// GenerateVideo now accepts a jobID string for better logging in parallel mode
func (s *Service) GenerateVideo(ctx context.Context, jobID string, modelName string, prompt string, cfg *genai.GenerateVideosConfig) (*genai.Video, error) {
	cleanName := strings.TrimPrefix(modelName, "vertexai/")

	fmt.Printf("[%s] 🚀 Sending request...\n", jobID)
	op, err := s.veoClient.Models.GenerateVideos(ctx, cleanName, prompt, nil, cfg)
	if err != nil {
		return nil, fmt.Errorf("start failed: %w", err)
	}
	fmt.Printf("[%s] ✅ Started: %s\n", jobID, op.Name)

	ticker := time.NewTicker(10 * time.Second) // Check every 10s to reduce noise
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			op, err = s.veoClient.Operations.GetVideosOperation(ctx, op, nil)
			if err != nil {
				return nil, fmt.Errorf("check failed: %w", err)
			}

			if op.Done {
				if op.Error != nil {
					return nil, fmt.Errorf("generation error: %v", op.Error)
				}
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
			// Optional: Print a dot or keep silent to avoid log clutter in parallel
			// fmt.Printf("[%s] ...\n", jobID)
		}
	}
}
