package processor

import (
	"testing"
)

func TestYouTubeProcessor_Name(t *testing.T) {
	p := NewYouTubeProcessor("/videos")
	if p.Name() != "youtube" {
		t.Errorf("Name() = %q, want %q", p.Name(), "youtube")
	}
}

func TestYouTubeProcessor_Match(t *testing.T) {
	p := NewYouTubeProcessor("/videos")

	tests := []struct {
		url  string
		want bool
	}{
		// YouTube URLs
		{"https://youtube.com/watch?v=abc123", true},
		{"https://www.youtube.com/watch?v=abc123", true},
		{"http://youtube.com/watch?v=abc123", true},
		{"http://www.youtube.com/watch?v=abc123", true},
		{"https://youtu.be/abc123", true},
		{"http://youtu.be/abc123", true},
		{"https://www.youtu.be/abc123", true},
		{"https://youtube.com/shorts/abc123", true},
		{"https://youtube.com/live/abc123", true},

		// Non-YouTube URLs
		{"https://vimeo.com/123456", false},
		{"https://example.com/video", false},
		{"https://notyoutube.com/watch", false},
		{"https://fakeyoutube.com/watch", false},
		{"youtube.com/watch?v=abc", false}, // missing protocol
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := p.Match(tt.url)
			if got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
