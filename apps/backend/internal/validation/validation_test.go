package validation

import (
	"strings"
	"testing"
)

func TestValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{email: "test@example.com", want: true},
		{email: "test+label@example.co.uk", want: true},
		{email: "missing-at.example.com", want: false},
		{email: "missing-host@", want: false},
		{email: "has space@example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := ValidEmail(tt.email); got != tt.want {
				t.Fatalf("ValidEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{"too short", strings.Repeat("a", 7), false},
		{"minimum length", strings.Repeat("a", 8), true},
		{"maximum length", strings.Repeat("a", 128), true},
		{"too long", strings.Repeat("a", 129), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidPassword(tt.password); got != tt.want {
				t.Fatalf("ValidPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}
