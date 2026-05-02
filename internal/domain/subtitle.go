package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Subtitle struct {
	ID        string
	SourceURL string
	AccessURL string
	Content   string
	CreatedAt time.Time
	ExpiresAt time.Time
	Valid     bool
}

var (
	ErrInvalidURL           = errors.New("invalid subtitle URL")
	ErrUnsupportedFormat    = errors.New("subtitle URL must point to .srt, .vtt or .webvtt content")
	ErrInvalidContentFormat = errors.New("subtitle content must be valid .srt, .vtt or .webvtt")
	ErrMaliciousContent     = errors.New("subtitle contains potentially malicious content")
	ErrEmptyContent         = errors.New("empty subtitle content")
)

var cueTimingLinePattern = regexp.MustCompile(`(?m)^\s*\d{2}:\d{2}:\d{2}(?:[\.,]\d{1,3})?\s+-->\s+\d{2}:\d{2}:\d{2}(?:[\.,]\d{1,3})?`)

func ValidateSubtitleURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return ErrInvalidURL
	}

	lowerPath := strings.ToLower(parsedURL.Path)
	if !strings.HasSuffix(lowerPath, ".srt") &&
		!strings.HasSuffix(lowerPath, ".vtt") &&
		!strings.HasSuffix(lowerPath, ".webvtt") {
		return ErrUnsupportedFormat
	}

	return nil
}

func ValidateSubtitleContent(body []byte) error {
	content := strings.TrimSpace(string(body))
	if content == "" {
		return ErrEmptyContent
	}
	if LooksMalicious(content) {
		return ErrMaliciousContent
	}
	if !isSupportedSubtitleContent(content) {
		return ErrInvalidContentFormat
	}
	return nil
}

func isSupportedSubtitleContent(content string) bool {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	trimmed := strings.TrimSpace(normalized)

	if strings.HasPrefix(trimmed, "WEBVTT") {
		return cueTimingLinePattern.MatchString(trimmed)
	}

	return cueTimingLinePattern.MatchString(trimmed)
}

func LooksMalicious(content string) bool {
	lowerContent := strings.ToLower(content)
	for _, marker := range []string{"<script", "javascript:", "<iframe", "onerror=", "onload="} {
		if strings.Contains(lowerContent, marker) {
			return true
		}
	}
	return false
}

func GenerateIdentifier() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
