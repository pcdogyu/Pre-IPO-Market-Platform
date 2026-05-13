package security

import "testing"

func TestHashPasswordAndCheckPassword(t *testing.T) {
	const password = "demo123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	if hash == password {
		t.Fatal("hash should not equal the plain password")
	}
	if !CheckPassword(hash, password) {
		t.Fatal("password should match generated hash")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("wrong password should not match generated hash")
	}
}

func TestCheckPasswordRejectsInvalidHash(t *testing.T) {
	if CheckPassword("not-a-bcrypt-hash", "demo123") {
		t.Fatal("invalid bcrypt hash should not validate")
	}
}
