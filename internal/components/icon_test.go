package components

import (
	"strings"
	"testing"
)

func TestIconSVG(t *testing.T) {
	svg := string(Icon("rocket").SVG())
	if !strings.HasPrefix(svg, `<svg aria-hidden="true"`) || !strings.Contains(svg, "<path") || strings.Contains(svg, "license") {
		t.Errorf("rocket icon = %q", svg)
	}
	if got := Icon("does-not-exist").SVG(); got != "" {
		t.Errorf("unknown icon = %q, want empty", got)
	}
}
