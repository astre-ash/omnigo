package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "positive integer float",
			input:    123.0,
			expected: "123",
		},
		{
			name:     "fractional number",
			input:    123.456,
			expected: "123.456",
		},
		{
			name:     "zero",
			input:    0.0,
			expected: "0",
		},
		{
			name:     "negative float",
			input:    -42.5,
			expected: "-42.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := formatFloat(tt.input)
			assert.Equal(t, tt.expected, actual, "formatted float mismatch")
		})
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "positive int",
			input:    42,
			expected: "42",
		},
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "negative int",
			input:    -100,
			expected: "-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := formatInt(tt.input)
			assert.Equal(t, tt.expected, actual, "formatted int mismatch")
		})
	}
}
