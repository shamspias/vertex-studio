package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"golang.org/x/oauth2/google"
)

// Client handles the interaction with Vertex AI via REST
type Client struct {
	ProjectID string
	Location  string
	ModelID   string
	Token     string
}

// Request/Response Structures for Veo 3.1
type VeoRequest struct {
	Instances  []VeoInstance `json:"instances"`
	Parameters VeoParams     `json:"parameters"`
}

type VeoInstance struct {
	Prompt string `json:"prompt"`
}

type VeoParams struct {
	AspectRatio      string `json:"aspectRatio"`
	DurationSeconds  int    `json:"durationSeconds"`
	PersonGeneration string `json:"personGeneration"`
}

type OperationResponse struct {
	Name string `json:"name"` // The Operation ID
}

type PollResponse struct {
	Done     bool `json:"done"`
	Response struct {
		Videos []struct {
			Uri      string `json:"gcsUri"`
			MimeType string `json:"mimeType"`
		} `json:"videos"`
	} `json:"response"`
}

func NewClient(ctx context.Context) (*Client, error) {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")

	if projectID == "" || location == "" {
		return nil, fmt.Errorf("missing config: GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION must be set in .env")
	}

	// 1. Get Access Token (Standard Google Auth)
	// This works with both 'gcloud auth login' AND Service Account Keys
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("auth error: %v", err)
	}
	token, err := creds.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("token error: %v", err)
	}

	return &Client{
		ProjectID: projectID,
		Location:  location,
		ModelID:   "veo-3.1-generate-preview", // or veo-3.1-generate-001
		Token:     token.AccessToken,
	}, nil
}

// GenerateVideo executes the REST call to predictLongRunning
func (c *Client) GenerateVideo(ctx context.Context, prompt string, startImagePath string, outputFilename string) error {
	endpoint := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predictLongRunning",
		c.Location, c.ProjectID, c.Location, c.ModelID)

	fmt.Printf("   -> [Vertex REST] Sending Job to %s...\n", c.ModelID)

	// 1. Construct Payload (Matches your curl JSON)
	payload := VeoRequest{
		Instances: []VeoInstance{
			{Prompt: prompt},
		},
		Parameters: VeoParams{
			AspectRatio:      "16:9",
			DurationSeconds:  8, // Forced to 8 for Veo 3.1
			PersonGeneration: "allow_all",
		},
	}

	jsonData, _ := json.Marshal(payload)

	// 2. Make Request
	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API Error (%d): %s", resp.StatusCode, string(body))
	}

	// 3. Get Operation ID
	var opRes OperationResponse
	json.NewDecoder(resp.Body).Decode(&opRes)
	fmt.Printf("   -> [Vertex] Operation Started: %s\n", opRes.Name)

	// 4. Poll for Completion
	videoURI, err := c.pollOperation(opRes.Name)
	if err != nil {
		return err
	}

	// 5. Download Video
	fmt.Printf("   -> [Download] Saving to %s...\n", outputFilename)
	return downloadFromGCS(videoURI, outputFilename)
}

func (c *Client) pollOperation(operationName string) (string, error) {
	endpoint := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s:fetchPredictOperation", c.Location, operationName)

	client := &http.Client{}

	for {
		fmt.Print(".") // Loading indicator
		time.Sleep(5 * time.Second)

		req, _ := http.NewRequest("POST", endpoint, nil) // fetch doesn't need body, just the name in URL or empty json
		req.Header.Set("Authorization", "Bearer "+c.Token)

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}

		var pollRes PollResponse
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		json.Unmarshal(bodyBytes, &pollRes)

		if pollRes.Done {
			fmt.Println(" Done!")
			if len(pollRes.Response.Videos) > 0 {
				return pollRes.Response.Videos[0].Uri, nil
			}
			return "", fmt.Errorf("operation done but no video found in response")
		}
	}
}

// Helper to download using GCloud CLI (Most reliable method without heavy deps)
func downloadFromGCS(gcsURI, localPath string) error {
	cmd := exec.Command("gcloud", "storage", "cp", gcsURI, localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud cp failed: %s", string(output))
	}
	return nil
}
