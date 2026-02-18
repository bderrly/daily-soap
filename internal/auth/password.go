// Package auth provides password hashing and verification using Argon2id with bcrypt fallback.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidHash is returned when the encoded hash is not in the correct format.
	ErrInvalidHash = errors.New("the encoded hash is not in the correct format")
	// ErrIncompatibleVersion is returned when the argon2 version is incompatible.
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
)

type params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// OWASP recommendations (m=19456 (19 MiB), t=2, p=1).
var defaultParams = params{
	memory:      19456,
	iterations:  2,
	parallelism: 1,
	saltLength:  16,
	keyLength:   32,
}

// HashPassword creates an Argon2id hash of the password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, defaultParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("reading random salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, defaultParams.iterations, defaultParams.memory, defaultParams.parallelism, defaultParams.keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, defaultParams.memory, defaultParams.iterations, defaultParams.parallelism, b64Salt, b64Hash)

	return encodedHash, nil
}

// VerifyPassword checks if the password matches the hash.
func VerifyPassword(password, encodedHash string) (match bool, needsUpgrade bool, err error) {
	if strings.HasPrefix(encodedHash, "$argon2id$") {
		p, salt, hash, err := decodeHash(encodedHash)
		if err != nil {
			if errors.Is(err, ErrInvalidHash) {
				return false, false, nil
			}
			return false, false, err
		}

		otherHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

		if subtle.ConstantTimeCompare(hash, otherHash) != 1 {
			return false, false, nil
		}

		// Check if parameters match current defaults.
		needsUpgrade = p.memory != defaultParams.memory ||
			p.iterations != defaultParams.iterations ||
			p.parallelism != defaultParams.parallelism ||
			p.saltLength != defaultParams.saltLength ||
			p.keyLength != defaultParams.keyLength

		return true, needsUpgrade, nil
	}

	// Fallback to bcrypt.
	err = bcrypt.CompareHashAndPassword([]byte(encodedHash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("bcrypt comparison: %w", err)
	}
	return true, true, nil
}

func decodeHash(encodedHash string) (p *params, salt, hash []byte, err error) {
	if !strings.HasPrefix(encodedHash, "$argon2id$") {
		return nil, nil, nil, ErrInvalidHash
	}

	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	_, err = fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing version from %q: %w", vals[2], err)
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	p = &params{}
	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing parameters from %q: %w", vals[3], err)
	}

	salt, err = base64.RawStdEncoding.DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decoding salt %q: %w", vals[4], err)
	}
	saltLen := uint64(len(salt))
	if saltLen > math.MaxUint32 {
		return nil, nil, nil, fmt.Errorf("salt too long")
	}
	p.saltLength = uint32(saltLen)

	hash, err = base64.RawStdEncoding.DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decoding hash %q: %w", vals[5], err)
	}
	hashLen := uint64(len(hash))
	if hashLen > math.MaxUint32 {
		return nil, nil, nil, fmt.Errorf("hash too long")
	}
	p.keyLength = uint32(hashLen)

	return p, salt, hash, nil
}
