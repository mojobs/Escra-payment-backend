package utils

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("securePass123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if hash == "securePass123" {
		t.Fatalf("password hash should not equal plaintext")
	}

	if err := CheckPassword(hash, "securePass123"); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}

	if err := CheckPassword(hash, "wrongPassword"); err == nil {
		t.Fatalf("invalid password accepted")
	}
}
