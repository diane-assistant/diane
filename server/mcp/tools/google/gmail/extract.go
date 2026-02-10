package gmail

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/gmail/v1"
)

// ExtractedContent holds extracted content from an email
type ExtractedContent struct {
	PlainText string `json:"plain_text"`
	HTML      string `json:"html"`
	JsonLD    []any  `json:"json_ld,omitempty"`
}

// ExtractContent extracts plain text and JSON-LD from a Gmail message
func ExtractContent(msg *gmail.Message) (*ExtractedContent, error) {
	if msg.Payload == nil {
		return &ExtractedContent{}, nil
	}

	content := &ExtractedContent{}

	// Extract text parts
	extractParts(msg.Payload, content)

	// If we have HTML but no plain text, convert HTML to plain text
	if content.PlainText == "" && content.HTML != "" {
		content.PlainText = htmlToPlainText(content.HTML)
	}

	// Extract JSON-LD from HTML
	if content.HTML != "" {
		content.JsonLD = extractJsonLD(content.HTML)
	}

	return content, nil
}

// extractParts recursively extracts text content from message parts
func extractParts(part *gmail.MessagePart, content *ExtractedContent) {
	if part == nil {
		return
	}

	mimeType := strings.ToLower(part.MimeType)

	// Handle body data
	if part.Body != nil && part.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err == nil {
			switch {
			case mimeType == "text/plain":
				content.PlainText = string(data)
			case mimeType == "text/html":
				content.HTML = string(data)
			}
		}
	}

	// Recurse into multipart
	for _, subPart := range part.Parts {
		extractParts(subPart, content)
	}
}

// htmlToPlainText converts HTML to plain text
func htmlToPlainText(html string) string {
	// Remove script and style elements
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	text := reScript.ReplaceAllString(html, "")
	text = reStyle.ReplaceAllString(text, "")

	// Convert common block elements to newlines
	reBlock := regexp.MustCompile(`(?i)</(p|div|tr|li|h[1-6]|br)[^>]*>`)
	text = reBlock.ReplaceAllString(text, "\n")

	// Remove all remaining HTML tags
	reTags := regexp.MustCompile(`<[^>]+>`)
	text = reTags.ReplaceAllString(text, "")

	// Decode HTML entities
	text = decodeHTMLEntities(text)

	// Clean up whitespace
	reSpaces := regexp.MustCompile(`[ \t]+`)
	text = reSpaces.ReplaceAllString(text, " ")

	// Clean up multiple newlines
	reNewlines := regexp.MustCompile(`\n\s*\n+`)
	text = reNewlines.ReplaceAllString(text, "\n\n")

	// Trim
	text = strings.TrimSpace(text)

	return text
}

// extractJsonLD extracts JSON-LD blocks from HTML
func extractJsonLD(html string) []any {
	// Match <script type="application/ld+json">...</script>
	re := regexp.MustCompile(`(?is)<script[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
	matches := re.FindAllStringSubmatch(html, -1)

	var results []any

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		jsonStr := strings.TrimSpace(match[1])
		if jsonStr == "" {
			continue
		}

		// Try to parse as JSON
		var data any
		if err := json.Unmarshal([]byte(jsonStr), &data); err == nil {
			// Could be a single object or an array
			if arr, ok := data.([]any); ok {
				results = append(results, arr...)
			} else {
				results = append(results, data)
			}
		}
	}

	return results
}

// GetJsonLDTypes extracts @type values from JSON-LD data
func GetJsonLDTypes(jsonLD []any) []string {
	types := make(map[string]bool)

	for _, item := range jsonLD {
		extractTypes(item, types)
	}

	result := make([]string, 0, len(types))
	for t := range types {
		result = append(result, t)
	}

	return result
}

// extractTypes recursively extracts @type values
func extractTypes(data any, types map[string]bool) {
	switch v := data.(type) {
	case map[string]any:
		if t, ok := v["@type"].(string); ok {
			types[t] = true
		}
		// Also check for array of types
		if arr, ok := v["@type"].([]any); ok {
			for _, t := range arr {
				if s, ok := t.(string); ok {
					types[s] = true
				}
			}
		}
		// Recurse into nested objects
		for _, val := range v {
			extractTypes(val, types)
		}
	case []any:
		for _, item := range v {
			extractTypes(item, types)
		}
	}
}

// decodeHTMLEntities decodes common HTML entities
func decodeHTMLEntities(s string) string {
	entities := map[string]string{
		"&nbsp;":   " ",
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   `"`,
		"&apos;":   "'",
		"&#39;":    "'",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&copy;":   "©",
		"&reg;":    "®",
		"&trade;":  "™",
		"&euro;":   "€",
		"&pound;":  "£",
		"&yen;":    "¥",
		"&cent;":   "¢",
		"&hellip;": "...",
		"&laquo;":  "«",
		"&raquo;":  "»",
		"&bull;":   "•",
		"&middot;": "·",
	}

	for entity, replacement := range entities {
		s = strings.ReplaceAll(s, entity, replacement)
	}

	// Decode numeric entities (&#123; or &#x7B;)
	reNumeric := regexp.MustCompile(`&#(\d+);`)
	s = reNumeric.ReplaceAllStringFunc(s, func(match string) string {
		var num int
		if _, err := fmt.Sscanf(match, "&#%d;", &num); err == nil && num > 0 && num < 65536 {
			return string(rune(num))
		}
		return match
	})

	reHex := regexp.MustCompile(`&#[xX]([0-9a-fA-F]+);`)
	s = reHex.ReplaceAllStringFunc(s, func(match string) string {
		var hex string
		if _, err := fmt.Sscanf(match, "&#x%s", &hex); err == nil {
			hex = strings.TrimSuffix(hex, ";")
			var num int
			if _, err := fmt.Sscanf(hex, "%x", &num); err == nil && num > 0 && num < 65536 {
				return string(rune(num))
			}
		}
		return match
	})

	return s
}

// SummarizeContent creates a token-efficient summary of email content
func SummarizeContent(content *ExtractedContent, maxLength int) string {
	if maxLength <= 0 {
		maxLength = 2000
	}

	text := content.PlainText
	if len(text) > maxLength {
		text = text[:maxLength] + "..."
	}

	return text
}

// HasJsonLD checks if the message contains JSON-LD data
func HasJsonLD(msg *gmail.Message) bool {
	if msg.Payload == nil {
		return false
	}

	var html string
	extractHTMLPart(msg.Payload, &html)

	if html == "" {
		return false
	}

	// Quick check without full parsing
	return strings.Contains(html, "application/ld+json")
}

// extractHTMLPart extracts just the HTML part for quick checks
func extractHTMLPart(part *gmail.MessagePart, html *string) {
	if part == nil || *html != "" {
		return
	}

	if strings.ToLower(part.MimeType) == "text/html" && part.Body != nil && part.Body.Data != "" {
		if data, err := base64.URLEncoding.DecodeString(part.Body.Data); err == nil {
			*html = string(data)
			return
		}
	}

	for _, subPart := range part.Parts {
		extractHTMLPart(subPart, html)
		if *html != "" {
			return
		}
	}
}
