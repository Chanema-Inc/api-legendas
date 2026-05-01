package domain

import (
	"errors"
	"testing"
)

func TestValidateSubtitleURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		errWant error
	}{
		{name: "valid vtt", input: "https://example.com/subtitle.vtt"},
		{name: "valid srt", input: "https://example.com/subtitle.srt"},
		{name: "invalid url", input: "://bad", errWant: ErrInvalidURL},
		{name: "unsupported extension", input: "https://example.com/subtitle.txt", errWant: ErrUnsupportedFormat},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSubtitleURL(test.input)
			if !errors.Is(err, test.errWant) {
				t.Fatalf("expected error %v, got %v", test.errWant, err)
			}
		})
	}
}

func TestValidateSubtitleContentRejectsMaliciousContent(t *testing.T) {
	t.Parallel()

	err := ValidateSubtitleContent([]byte("<script>alert(1)</script>"))
	if !errors.Is(err, ErrMaliciousContent) {
		t.Fatalf("expected ErrMaliciousContent, got %v", err)
	}
}

func TestValidateSubtitleContentRejectsEmptyContent(t *testing.T) {
	t.Parallel()

	err := ValidateSubtitleContent([]byte("   \n\t"))
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("expected ErrEmptyContent, got %v", err)
	}
}

func TestValidateSubtitleContentAcceptsValidContent(t *testing.T) {
	t.Parallel()

	err := ValidateSubtitleContent([]byte("1\n00:00:00,000 --> 00:00:01,500\nHello\n"))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestGenerateIdentifierReturnsHexValue(t *testing.T) {
	t.Parallel()

	id, err := GenerateIdentifier()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("expected 32-char hex id, got %q", id)
	}
}
