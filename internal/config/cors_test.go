package config

import "testing"

func TestValidateCORSOrigins(t *testing.T) {
	valid := []string{
		"https://app.example.com",
		"http://localhost:3000",
		"https://app.example.com,http://localhost:3000",
	}
	for _, v := range valid {
		if err := validateCORSOrigins(v); err != nil {
			t.Errorf("validateCORSOrigins(%q) = %v, want nil", v, err)
		}
	}

	invalid := []string{"", "*", "https://*.example.com", "app.example.com", "https://app.example.com/path", "https://a.com,,https://b.com"}
	for _, v := range invalid {
		if err := validateCORSOrigins(v); err == nil {
			t.Errorf("validateCORSOrigins(%q) = nil, want error", v)
		}
	}
}
