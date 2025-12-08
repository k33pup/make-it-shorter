package validator

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	shortCodeRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,10}$`)
)

func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	if len(urlStr) > 2048 {
		return fmt.Errorf("URL too long (max 2048 characters)")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	// Prevent potential SSRF attacks
	if strings.Contains(parsedURL.Host, "localhost") ||
	   strings.Contains(parsedURL.Host, "127.0.0.1") ||
	   strings.Contains(parsedURL.Host, "0.0.0.0") {
		return fmt.Errorf("localhost URLs are not allowed")
	}

	return nil
}

func ValidateShortCode(code string) error {
	if code == "" {
		return fmt.Errorf("short code cannot be empty")
	}

	if !shortCodeRegex.MatchString(code) {
		return fmt.Errorf("short code must be 3-10 characters, alphanumeric, dash or underscore only")
	}

	return nil
}

func SanitizeInput(input string) string {
	// Remove potential XSS characters
	input = strings.ReplaceAll(input, "<", "")
	input = strings.ReplaceAll(input, ">", "")
	input = strings.ReplaceAll(input, "\"", "")
	input = strings.ReplaceAll(input, "'", "")
	input = strings.TrimSpace(input)
	return input
}
