package services

import (
	"testing"
)

func TestPIIDetector_Detect(t *testing.T) {
	detector := NewPIIDetector()

	tests := []struct {
		name          string
		text          string
		expectedTypes []string
	}{
		{
			name:          "detect Chinese phone",
			text:          "My phone is 13812345678",
			expectedTypes: []string{"phone"},
		},
		{
			name:          "detect Chinese ID card",
			text:          "ID: 110101199001011234",
			expectedTypes: []string{"id_card"},
		},
		{
			name:          "detect email",
			text:          "Email: test@example.com",
			expectedTypes: []string{"email"},
		},
		{
			name:          "detect bank card",
			text:          "Card: 1234567890123456",
			expectedTypes: []string{"bank_card"},
		},
		{
			name:          "detect multiple PII types",
			text:          "Phone: 13987654321, Email: user@test.org",
			expectedTypes: []string{"phone", "email"},
		},
		{
			name:          "no PII detected",
			text:          "This is a normal text without PII",
			expectedTypes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detections := detector.Detect(tt.text)
			if len(detections) != len(tt.expectedTypes) {
				t.Errorf("Expected %d detections, got %d", len(tt.expectedTypes), len(detections))
			}
			for i, expectedType := range tt.expectedTypes {
				if i < len(detections) && detections[i].Type != expectedType {
					t.Errorf("Expected type %s, got %s", expectedType, detections[i].Type)
				}
			}
		})
	}
}

func TestPIIDetector_Mask(t *testing.T) {
	detector := NewPIIDetector()

	tests := []struct {
		name     string
		text     string
		detection Detection
		expected  string
	}{
		{
			name:     "mask phone",
			text:     "13812345678",
			detection: Detection{Type: "phone"},
			expected: "138****5678",
		},
		{
			name:     "mask ID card",
			text:     "110101199001011234",
			detection: Detection{Type: "id_card"},
			expected: "110101********1234",
		},
		{
			name:     "mask email",
			text:     "test@example.com",
			detection: Detection{Type: "email"},
			expected: "te***@example.com",
		},
		{
			name:     "mask bank card",
			text:     "1234567890123456",
			detection: Detection{Type: "bank_card"},
			expected: "1234****3456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Mask(tt.text, tt.detection)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestPIIDetector_DetectAndMask(t *testing.T) {
	detector := NewPIIDetector()

	tests := []struct {
		name              string
		text               string
		expectMasked       bool
		expectedDetectionCount int
	}{
		{
			name:              "mask phone in text",
			text:              "Call me at 13812345678",
			expectMasked:      true,
			expectedDetectionCount: 1,
		},
		{
			name:              "mask multiple PII",
			text:              "Phone: 13900001111, ID: 110101199001011234",
			expectMasked:      true,
			expectedDetectionCount: 2,
		},
		{
			name:              "no masking needed",
			text:              "Hello world",
			expectMasked:      false,
			expectedDetectionCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked, detections := detector.DetectAndMask(tt.text)
			if len(detections) != tt.expectedDetectionCount {
				t.Errorf("Expected %d detections, got %d", tt.expectedDetectionCount, len(detections))
			}
			if tt.expectMasked && masked == tt.text {
				t.Errorf("Expected text to be masked, but it wasn't")
			}
			if !tt.expectMasked && masked != tt.text {
				t.Errorf("Expected text to remain unchanged, but got %s", masked)
			}
		})
	}
}
