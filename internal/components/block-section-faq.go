package components

import "storyblok-go-website/internal/storyblok"

type BlockSectionFaq struct {
	storyblok.Blok
	SectionStyle
	Heading string `json:"heading"`
	Items   Blocks `json:"items"`
}

// StructuredData describes the answered questions as a schema.org FAQPage,
// or nil if there are none.
func (f *BlockSectionFaq) StructuredData() map[string]any {
	var questions []map[string]any
	for _, item := range f.Items {
		q, ok := item.(*BlockContentQuestion)
		if !ok || q.Question == "" || q.Answer == "" {
			continue
		}
		questions = append(questions, map[string]any{
			"@type":          "Question",
			"name":           q.Question,
			"acceptedAnswer": map[string]any{"@type": "Answer", "text": q.Answer},
		})
	}
	if len(questions) == 0 {
		return nil
	}
	return map[string]any{"@context": "https://schema.org", "@type": "FAQPage", "mainEntity": questions}
}
