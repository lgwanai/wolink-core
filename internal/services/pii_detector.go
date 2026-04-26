package services

import (
	"regexp"
	"sync"
)

// Detection represents a single PII detection
type Detection struct {
	Type     string `json:"type"`     // phone, id_card, email, bank_card
	Start    int    `json:"start"`    // Start position in text
	End      int    `json:"end"`      // End position in text
	Matched  string `json:"matched"`  // The matched text
	Masked   string `json:"masked"`   // The masked version
}

// PIIDetector detects and masks PII in text
type PIIDetector struct {
	phonePattern    *regexp.Regexp
	idCardPattern   *regexp.Regexp
	emailPattern    *regexp.Regexp
	bankCardPattern *regexp.Regexp
}

var (
	detectorOnce sync.Once
	detector     *PIIDetector
)

// NewPIIDetector creates a PII detector with pre-compiled patterns
func NewPIIDetector() *PIIDetector {
	detectorOnce.Do(func() {
		detector = &PIIDetector{
			// Chinese mobile phone: 1[3-9]xxxxxxxxx (11 digits)
			phonePattern: regexp.MustCompile(`1[3-9]\d{9}`),
			// Chinese ID card: 18 digits, last can be X
			idCardPattern: regexp.MustCompile(`[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
			// Email: RFC 5322 simplified
			emailPattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			// Bank card: 16-19 digits
			bankCardPattern: regexp.MustCompile(`\d{16,19}`),
		}
	})
	return detector
}

// Detect finds all PII in the given text
func (d *PIIDetector) Detect(text string) []Detection {
	var detections []Detection

	// Detect phones
	detections = append(detections, d.detectPattern(text, "phone", d.phonePattern)...)
	// Detect ID cards
	detections = append(detections, d.detectPattern(text, "id_card", d.idCardPattern)...)
	// Detect emails
	detections = append(detections, d.detectPattern(text, "email", d.emailPattern)...)
	// Detect bank cards (after other patterns to avoid overlapping with ID cards)
	detections = append(detections, d.detectPattern(text, "bank_card", d.bankCardPattern)...)

	// Sort by start position and remove overlapping detections
	return d.removeOverlaps(detections)
}

// detectPattern finds all matches of a pattern in text
func (d *PIIDetector) detectPattern(text string, piiType string, pattern *regexp.Regexp) []Detection {
	var detections []Detection
	matches := pattern.FindAllStringIndex(text, -1)

	for _, match := range matches {
		start, end := match[0], match[1]
		matched := text[start:end]
		detections = append(detections, Detection{
			Type:    piiType,
			Start:   start,
			End:     end,
			Matched: matched,
			Masked:  d.Mask(matched, Detection{Type: piiType}),
		})
	}
	return detections
}

// Mask replaces text with asterisks, preserving length
func (d *PIIDetector) Mask(text string, detection Detection) string {
	runes := []rune(text)
	length := len(runes)

	switch detection.Type {
	case "phone":
		// Show first 3 and last 4: 138****1234
		if length >= 7 {
			return string(runes[:3]) + "****" + string(runes[length-4:])
		}
	case "id_card":
		// Show first 6 and last 4: 110101********1234
		if length >= 10 {
			return string(runes[:6]) + "********" + string(runes[length-4:])
		}
	case "email":
		// Show first 2 chars and domain: ab***@example.com
		atIndex := -1
		for i, r := range runes {
			if r == '@' {
				atIndex = i
				break
			}
		}
		if atIndex > 2 {
			return string(runes[:2]) + "***" + string(runes[atIndex:])
		}
	case "bank_card":
		// Show first 4 and last 4: 1234********5678
		if length >= 8 {
			return string(runes[:4]) + "****" + string(runes[length-4:])
		}
	}

	// Default: mask all
	masked := make([]rune, length)
	for i := range masked {
		masked[i] = '*'
	}
	return string(masked)
}

// DetectAndMask detects PII and returns masked text with detection metadata
func (d *PIIDetector) DetectAndMask(text string) (string, []Detection) {
	detections := d.Detect(text)
	if len(detections) == 0 {
		return text, nil
	}

	// Build masked text from right to left to preserve positions
	result := []rune(text)
	for i := len(detections) - 1; i >= 0; i-- {
		d := detections[i]
		masked := []rune(d.Masked)
		start := d.Start
		for j, m := range masked {
			if start+j < len(result) {
				result[start+j] = m
			}
		}
	}

	return string(result), detections
}

// removeOverlaps removes overlapping detections, keeping the first one
func (d *PIIDetector) removeOverlaps(detections []Detection) []Detection {
	if len(detections) <= 1 {
		return detections
	}

	// Sort by start position
	for i := 0; i < len(detections)-1; i++ {
		for j := i + 1; j < len(detections); j++ {
			if detections[i].Start > detections[j].Start {
				detections[i], detections[j] = detections[j], detections[i]
			}
		}
	}

	// Remove overlaps
	var result []Detection
	for _, d := range detections {
		overlaps := false
		for _, existing := range result {
			if d.Start < existing.End && d.End > existing.Start {
				overlaps = true
				break
			}
		}
		if !overlaps {
			result = append(result, d)
		}
	}

	return result
}
