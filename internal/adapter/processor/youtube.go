package processor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/cwygoda/catcher/internal/domain"
)

var youtubePattern = regexp.MustCompile(`^https?://(www\.)?(youtube\.com|youtu\.be)/`)

// YouTubeProcessor downloads videos using yt-dlp.
type YouTubeProcessor struct {
	videoDir string
}

// NewYouTubeProcessor creates a new YouTube processor.
func NewYouTubeProcessor(videoDir string) *YouTubeProcessor {
	return &YouTubeProcessor{videoDir: videoDir}
}

// Name returns the processor name.
func (p *YouTubeProcessor) Name() string {
	return "youtube"
}

// Match returns true if the URL is a YouTube URL.
func (p *YouTubeProcessor) Match(url string) bool {
	return youtubePattern.MatchString(url)
}

// Process downloads the video using yt-dlp to a temp dir, then moves to final dir.
func (p *YouTubeProcessor) Process(ctx context.Context, job *domain.Job) error {
	// Create temp directory for this download
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("catcher-job-%d-*", job.ID))
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up on any exit

	// Download to temp directory
	outputTemplate := filepath.Join(tempDir, "%(title)s.%(ext)s")
	cmd := exec.CommandContext(ctx, "yt-dlp", "-o", outputTemplate, job.URL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yt-dlp failed: %w: %s", err, string(output))
	}

	// Move downloaded files to final directory
	if err := p.moveFiles(tempDir); err != nil {
		return fmt.Errorf("move files: %w", err)
	}

	return nil
}

// moveFiles moves all files from src directory to the video directory.
func (p *YouTubeProcessor) moveFiles(srcDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	// Ensure video directory exists
	if err := os.MkdirAll(p.videoDir, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(p.videoDir, entry.Name())

		// Rename (move) the file
		if err := os.Rename(src, dst); err != nil {
			// If rename fails (cross-device), fall back to copy+delete
			if err := copyFile(src, dst); err != nil {
				return err
			}
			os.Remove(src)
		}
	}
	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
