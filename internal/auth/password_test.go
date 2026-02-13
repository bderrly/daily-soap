package auth_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"

	"derrclan.com/moravian-soap/internal/auth"
)

func TestHashAndVerify(t *testing.T) {
	password := "correct-horse-battery-staple"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	match, needsUpgrade, err := auth.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("failed to verify password: %v", err)
	}
	if !match {
		t.Error("expected password to match hash")
	}
	if needsUpgrade {
		t.Error("expected argon2id hash not to need upgrade")
	}

	match, _, err = auth.VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("failed to verify password: %v", err)
	}
	if match {
		t.Error("expected wrong password not to match hash")
	}
}

func TestBcryptMigration(t *testing.T) {
	password := "legacy-password"
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	match, needsUpgrade, err := auth.VerifyPassword(password, string(bcryptHash))
	if err != nil {
		t.Fatalf("failed to verify bcrypt password: %v", err)
	}
	if !match {
		t.Error("expected legacy password to match bcrypt hash")
	}
	if !needsUpgrade {
		t.Error("expected bcrypt hash to need upgrade")
	}
}

func TestArgon2Upgrade(t *testing.T) {
	password := "upgrade-me"
	salt := []byte("somesaltlongerthan16")

	// Generate a hash with 1 iteration (default is 2)
	hashBytes := argon2.IDKey([]byte(password), salt, 1, 19456, 1, 32)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hashBytes)
	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, 19456, 1, 1, b64Salt, b64Hash)

	match, needsUpgrade, err := auth.VerifyPassword(password, encodedHash)
	if err != nil {
		t.Fatalf("failed to verify: %v", err)
	}
	if !match {
		t.Error("expected password to match")
	}
	if !needsUpgrade {
		t.Error("expected argon2id hash with t=1 to need upgrade")
	}
}

func TestParameterUpgrades(t *testing.T) {
	password := "upgrade-me"

	tests := []struct {
		name string
		m    uint32
		t    uint32
		p    uint8
		salt uint32
		key  uint32
	}{
		{"memory", 1024, 2, 1, 16, 32},
		{"parallelism", 19456, 2, 2, 16, 32},
		{"salt length", 19456, 2, 1, 32, 32},
		{"key length", 19456, 2, 1, 16, 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			salt := make([]byte, tt.salt)
			hashBytes := argon2.IDKey([]byte(password), salt, tt.t, tt.m, tt.p, tt.key)

			b64Salt := base64.RawStdEncoding.EncodeToString(salt)
			b64Hash := base64.RawStdEncoding.EncodeToString(hashBytes)
			encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, tt.m, tt.t, tt.p, b64Salt, b64Hash)

			match, needsUpgrade, err := auth.VerifyPassword(password, encodedHash)
			if err != nil {
				t.Fatalf("failed to verify %s: %v", tt.name, err)
			}
			if !match {
				t.Errorf("expected password to match for %s", tt.name)
			}
			if !needsUpgrade {
				t.Errorf("expected argon2id hash with %s mismatch to need upgrade", tt.name)
			}
		})
	}
}

func TestEmptyPassword(t *testing.T) {
	password := ""
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash empty password: %v", err)
	}

	match, _, err := auth.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("failed to verify empty password: %v", err)
	}
	if !match {
		t.Error("expected empty password to match")
	}
}

func TestArgon2IncompatibleVersion(t *testing.T) {
	// argon2.Version is 19
	incompatibleHash := "$argon2id$v=18$m=19456,t=2,p=1$c29tZXNhbHQ$c29tZWhhc2g"
	match, _, err := auth.VerifyPassword("password", incompatibleHash)
	if err == nil {
		t.Error("expected error for incompatible version, got nil")
	}
	if match {
		t.Error("expected match to be false")
	}
}

func TestInvalidArgonHashes(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"too few segments", "$argon2id$v=19$m=19456,t=2,p=1$salt"},
		{"invalid version format", "$argon2id$v=nan$m=19456,t=2,p=1$salt$hash"},
		{"invalid params format", "$argon2id$v=19$m=19456,t=2,p=nan$salt$hash"},
		{"invalid salt b64", "$argon2id$v=19$m=19456,t=2,p=1$!!$hash"},
		{"invalid hash b64", "$argon2id$v=19$m=19456,t=2,p=1$salt$!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, _, err := auth.VerifyPassword("password", tt.hash)
			if err != nil {
				// We expect error or no match depending on how it fails.
				// Currently VerifyPassword returns false, false, nil for ErrInvalidHash (too few segments).
				// For others it might return error from Sscanf or DecodeString.
				return
			}
			if match {
				t.Errorf("expected no match for %s", tt.name)
			}
		})
	}
}

func TestNonHashStrings(t *testing.T) {
	match, _, err := auth.VerifyPassword("password", "completely-random-string")
	// We don't strictly care if it returns an error or just match=false,
	// as long as it doesn't match.
	if match {
		t.Error("expected no match for random string")
	}
	// Currently it returns an error from bcrypt because it falls back to it.
	if err == nil {
		t.Log("Note: VerifyPassword returned nil error for random string")
	}
}
