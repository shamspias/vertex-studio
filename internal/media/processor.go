package media

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExtractLastFrame gets the very last frame of a video to ensure continuity.
func ExtractLastFrame(videoPath string) (string, error) {
	outputPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + "_last.jpg"

	// FFmpeg command: seek to end, extract 1 frame
	// -sseof -0.1 : Seek to 0.1 seconds before end
	// -update 1 : distinct image file
	cmd := exec.Command("ffmpeg", "-y", "-sseof", "-0.1", "-i", videoPath, "-vframes", "1", "-q:v", "2", outputPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %s : %s", err, string(output))
	}

	return outputPath, nil
}

// StitchVideos concatenates multiple MP4 files into one.
func StitchVideos(videoFiles []string, outputFile string) error {
	if len(videoFiles) == 0 {
		return fmt.Errorf("no videos to stitch")
	}

	// Create a temporary file list for ffmpeg
	listFile := "ffmpeg_concat_list.txt"
	f, err := os.Create(listFile)
	if err != nil {
		return err
	}

	for _, file := range videoFiles {
		// absolute path is safer for ffmpeg
		absPath, _ := filepath.Abs(file)
		f.WriteString(fmt.Sprintf("file '%s'\n", absPath))
	}
	f.Close()
	defer os.Remove(listFile)

	// FFmpeg concat command
	cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", listFile, "-c", "copy", outputFile)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stitch error: %s : %s", err, string(output))
	}

	return nil
}
