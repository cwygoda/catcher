package processor

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cwygoda/catcher/internal/config"
	"github.com/cwygoda/catcher/internal/domain"
)

// CommandProcessor runs an external command for matching URLs.
type CommandProcessor struct {
	name      string
	pattern   *regexp.Regexp
	command   string
	args      []string
	targetDir string
	isolate   bool
}

// NewCommandProcessor creates a processor from config.
// Uses default ~/Videos if target_dir not set, isolate defaults to true.
func NewCommandProcessor(pc config.ProcessorConfig) (*CommandProcessor, error) {
	re, err := regexp.Compile(pc.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pc.Pattern, err)
	}

	targetDir := pc.TargetDir
	if targetDir == "" {
		targetDir = config.DefaultTargetDir()
	} else {
		targetDir = config.ExpandPath(targetDir)
	}

	isolate := true
	if pc.Isolate != nil {
		isolate = *pc.Isolate
	}

	return &CommandProcessor{
		name:      pc.Name,
		pattern:   re,
		command:   pc.Command,
		args:      pc.Args,
		targetDir: targetDir,
		isolate:   isolate,
	}, nil
}

func (p *CommandProcessor) Name() string {
	return p.name
}

func (p *CommandProcessor) TargetDir() string {
	return p.targetDir
}

func (p *CommandProcessor) Match(url string) bool {
	return p.pattern.MatchString(url)
}

func (p *CommandProcessor) Process(ctx context.Context, job *domain.Job) error {
	// Build args with {url} placeholder replaced
	args := make([]string, len(p.args))
	for i, arg := range p.args {
		args[i] = strings.ReplaceAll(arg, "{url}", job.URL)
	}

	if p.isolate {
		return p.processIsolated(ctx, job, args)
	}
	return p.processDirect(ctx, args)
}

// processDirect runs command directly in target directory.
func (p *CommandProcessor) processDirect(ctx context.Context, args []string) error {
	if err := os.MkdirAll(p.targetDir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = p.targetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", p.command, err, string(output))
	}
	return nil
}

// processIsolated runs in temp dir, moves files on success.
func (p *CommandProcessor) processIsolated(ctx context.Context, job *domain.Job, args []string) error {
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("catcher-job-%d-*", job.ID))
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	log.Printf("job %d: running isolated in %s", job.ID, tempDir)
	defer os.RemoveAll(tempDir)

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", p.command, err, string(output))
	}

	return p.moveFiles(job.ID, tempDir)
}

// moveFiles moves files from src to target, skipping existing.
func (p *CommandProcessor) moveFiles(jobID int64, srcDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	// Collect file names for logging
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	log.Printf("job %d: found %d file(s): %v", jobID, len(files), files)

	if err := os.MkdirAll(p.targetDir, 0755); err != nil {
		return err
	}

	var moved []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(p.targetDir, entry.Name())

		// Skip if destination exists (no overwrite)
		if _, err := os.Stat(dst); err == nil {
			log.Printf("job %d: skipped %s (exists)", jobID, entry.Name())
			continue
		}

		if err := os.Rename(src, dst); err != nil {
			// Cross-device fallback
			if err := copyFile(src, dst); err != nil {
				return err
			}
			os.Remove(src)
		}
		moved = append(moved, entry.Name())
	}
	log.Printf("job %d: moved %d file(s) to %s", jobID, len(moved), p.targetDir)
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
