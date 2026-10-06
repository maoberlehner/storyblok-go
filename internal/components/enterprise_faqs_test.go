package components

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestEnterpriseFaqsNumbersQuestions(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	var faqs AnyBlock
	err = json.Unmarshal([]byte(`{"component": "enterprise_faqs", "faqs": [
		{"component": "faq_item", "question": "First?"},
		{"component": "faq_item", "question": "Second?"}
	]}`), &faqs)
	if err != nil {
		t.Fatal(err)
	}
	var html strings.Builder
	if err := r.Block(&html, faqs.Block); err != nil {
		t.Fatal(err)
	}
	first := strings.Index(html.String(), `<span class="faq-item__number">01</span>`)
	second := strings.Index(html.String(), `<span class="faq-item__number">02</span>`)
	if first < 0 || second < first {
		t.Errorf("questions are not numbered in order:\n%s", html.String())
	}
}
