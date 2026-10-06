package components

import "testing"

func TestFeatureShowcaseYouTubeEmbedURL(t *testing.T) {
	const embed = "https://www.youtube-nocookie.com/embed/q65Re45s-eE?autoplay=1&rel=0"
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.youtube.com/watch?v=q65Re45s-eE", embed},
		{"https://youtu.be/q65Re45s-eE", embed},
		{"https://www.youtube.com/embed/q65Re45s-eE", embed},
		{" https://m.youtube.com/watch?v=q65Re45s-eE&t=10 ", embed},
		{"https://vimeo.com/12345", ""},
		{"", ""},
	}
	for _, tt := range tests {
		f := &FeatureShowcase{YouTubeURL: tt.url}
		if got := f.YouTubeEmbedURL(); got != tt.want {
			t.Errorf("YouTubeEmbedURL() for %q = %q, want %q", tt.url, got, tt.want)
		}
	}
}
