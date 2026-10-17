package auth

import "testing"

func TestTokenRoundTrip(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	tok, err := NewToken(secret, 42)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseToken(secret, tok)
	if err != nil || id != 42 {
		t.Fatalf("got %d, %v", id, err)
	}
	if _, err := ParseToken("another-secret-another-secret-0000", tok); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "password123") || CheckPassword(h, "nope") {
		t.Fatal("password check failed")
	}
}
