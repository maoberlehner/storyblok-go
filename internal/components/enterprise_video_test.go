package components

import "testing"

func TestEnterpriseVideoYouTubeID(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.youtube.com/watch?v=KefIDv4KMnQ", "KefIDv4KMnQ"},
		{"https://youtu.be/KefIDv4KMnQ?t=10", "KefIDv4KMnQ"},
		{"https://www.youtube-nocookie.com/embed/KefIDv4KMnQ", "KefIDv4KMnQ"},
		{"https://youtube.com/shorts/KefIDv4KMnQ", "KefIDv4KMnQ"},
		{"https://example.com/watch?v=KefIDv4KMnQ", ""},
		{"https://www.youtube.com/watch?v=bad\"id", ""},
		{"", ""},
	}
	for _, tt := range tests {
		v := EnterpriseVideo{YoutubeURL: tt.url}
		if got := v.YouTubeID(); got != tt.want {
			t.Errorf("YouTubeID(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
