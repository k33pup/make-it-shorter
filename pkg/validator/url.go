package validator

import (
	"fmt"
	"net"
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

	// Prevent potential SSRF attacks - check for private/internal IPs
	if err := checkSSRF(parsedURL.Hostname()); err != nil {
		return err
	}

	return nil
}

// checkSSRF validates that the host is not a private/internal IP address
func checkSSRF(host string) error {
	// Check for localhost keywords
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") {
		return fmt.Errorf("localhost URLs are not allowed")
	}

	// Try to parse as IP address
	ip := net.ParseIP(host)
	if ip == nil {
		// If not an IP, try to resolve the hostname
		ips, err := net.LookupIP(host)
		if err != nil {
			// If we can't resolve, allow it (might be a valid domain that's temporarily unreachable)
			return nil
		}
		// Check all resolved IPs
		for _, resolvedIP := range ips {
			if err := isPrivateIP(resolvedIP); err != nil {
				return err
			}
		}
		return nil
	}

	// Check if the IP is private
	return isPrivateIP(ip)
}

// isPrivateIP checks if an IP address is private/internal
func isPrivateIP(ip net.IP) error {
	// Check for loopback (127.0.0.0/8 for IPv4, ::1 for IPv6)
	if ip.IsLoopback() {
		return fmt.Errorf("loopback IP addresses are not allowed")
	}

	// Check for private IP ranges
	if ip.IsPrivate() {
		return fmt.Errorf("private IP addresses are not allowed")
	}

	// Check for link-local addresses (169.254.0.0/16 for IPv4, fe80::/10 for IPv6)
	if ip.IsLinkLocalUnicast() {
		return fmt.Errorf("link-local IP addresses are not allowed")
	}

	// Check for multicast
	if ip.IsMulticast() {
		return fmt.Errorf("multicast IP addresses are not allowed")
	}

	// Check for unspecified (0.0.0.0 or ::)
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified IP addresses are not allowed")
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
	input = strings.TrimSpace(input)

	result := strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, input)

	replacements := map[string]string{
		"<":  "",
		">":  "",
		"\"": "",
		"'":  "",
		";":  "",
		"&":  "",
		"|":  "",
		"`":  "",
		"$":  "",
		"\n": "",
		"\r": "",
		"\t": "",
	}

	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

func ValidateAlphanumeric(input string, maxLength int) error {
	if len(input) > maxLength {
		return fmt.Errorf("input exceeds maximum length of %d", maxLength)
	}

	matched := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(input)
	if !matched {
		return fmt.Errorf("input contains invalid characters")
	}

	return nil
}

func SanitizeRedisKey(key string) string {
	var result strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		   (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ':' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func ValidateEmail(email string) error {
	if email == "" {
		return nil
	}

	if len(email) > 254 {
		return fmt.Errorf("email too long")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}
