// Package auth implements the single-password model: Argon2id hashing,
// server-side sessions and login throttling. It contains no HTTP code.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// MinPasswordLength is the minimum accepted password length (U-2).
const MinPasswordLength = 8

// Params are the Argon2id cost parameters stored alongside each hash, so the
// cost can be raised later without invalidating existing passwords.
type Params struct {
	Memory  uint32 // KiB
	Time    uint32 // iterations
	Threads uint8
	KeyLen  uint32
	SaltLen uint32
}

// DefaultParams are the production cost parameters. Changing them affects new
// hashes only; verification always uses the parameters stored in the hash.
var DefaultParams = Params{
	Memory:  64 * 1024, // 64 MiB
	Time:    3,
	Threads: 2,
	KeyLen:  32,
	SaltLen: 16,
}

// Hasher creates and verifies Argon2id password hashes. Tests use cheap
// parameters; production uses DefaultParams.
type Hasher struct {
	Params Params
}

// NewHasher returns a Hasher with the given parameters.
func NewHasher(params Params) *Hasher {
	return &Hasher{Params: params}
}

// Hash returns a PHC-formatted Argon2id hash:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
//
// The salt is random per call. The plain password is never stored.
func (h *Hasher) Hash(password string) (string, error) {
	if password == "" {
		return "", errors.New("auth: empty password")
	}
	salt := make([]byte, h.Params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: reading salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt,
		h.Params.Time, h.Params.Memory, h.Params.Threads, h.Params.KeyLen)

	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.Params.Memory, h.Params.Time, h.Params.Threads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify compares password with encoded in constant time. It returns false
// with no error for a wrong password, and an error only for a malformed hash.
func (h *Hasher) Verify(encoded, password string) (bool, error) {
	memory, iterations, threads, salt, want, err := parseHash(encoded)
	if err != nil {
		return false, err
	}
	// #nosec G115 -- the decoded hash length is bounded by the stored PHC string.
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func parseHash(encoded string) (memory, iterations uint32, threads uint8, salt, hash []byte, err error) {
	parts := strings.Split(encoded, "$")
	// Leading empty element, then algorithm, version, params, salt, hash.
	if len(parts) != 6 || parts[0] != "" {
		return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
	}
	if parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, fmt.Errorf("auth: unsupported algorithm %q", parts[1])
	}
	var version int
	if _, scanErr := fmt.Sscanf(parts[2], "v=%d", &version); scanErr != nil || version != argon2.Version {
		return 0, 0, 0, nil, nil, errors.New("auth: unsupported Argon2 version")
	}
	var threadsInt int
	if _, scanErr := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threadsInt); scanErr != nil {
		return 0, 0, 0, nil, nil, errors.New("auth: malformed Argon2 parameters")
	}
	if threadsInt < 1 || threadsInt > 255 {
		return 0, 0, 0, nil, nil, errors.New("auth: Argon2 parallelism out of range")
	}
	threads = uint8(threadsInt)
	enc := base64.RawStdEncoding
	salt, err = enc.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("auth: decoding salt: %w", err)
	}
	hash, err = enc.DecodeString(parts[5])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("auth: decoding hash: %w", err)
	}
	if len(salt) == 0 || len(hash) == 0 {
		return 0, 0, 0, nil, nil, errors.New("auth: empty salt or hash")
	}
	return memory, iterations, threads, salt, hash, nil
}
