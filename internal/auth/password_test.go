package auth

import (
	"errors"
	"strings"
	"testing"
)

func testHasher() *Hasher {
	return NewHasher(Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
}

func TestHashPassword_RoundTrip_U8(t *testing.T) {
	t.Parallel()

	h := testHasher()
	encoded, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash(): %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("hash %q is not a PHC Argon2id string", encoded)
	}
	if strings.Contains(encoded, "correct horse") {
		t.Fatal("hash contains the plain password")
	}

	ok, err := h.Verify(encoded, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !ok {
		t.Fatal("Verify() = false for the correct password")
	}

	ok, err = h.Verify(encoded, "wrong password")
	if err != nil {
		t.Fatalf("Verify(wrong): %v", err)
	}
	if ok {
		t.Fatal("Verify() = true for a wrong password")
	}
}

func TestHashPassword_SaltsDiffer(t *testing.T) {
	t.Parallel()

	h := testHasher()
	first, err := h.Hash("same password")
	if err != nil {
		t.Fatalf("Hash(): %v", err)
	}
	second, err := h.Hash("same password")
	if err != nil {
		t.Fatalf("Hash(): %v", err)
	}
	if first == second {
		t.Fatal("two hashes of the same password are identical; salt is not random")
	}
}

func TestVerify_RejectsMalformedHashes(t *testing.T) {
	t.Parallel()

	h := testHasher()
	tests := map[string]string{
		"empty":             "",
		"not enough parts":  "$argon2id$v=19$m=64,t=1,p=1$salt",
		"wrong algorithm":   "$argon2i$v=19$m=64,t=1,p=1$c2FsdA$aGFzaA",
		"wrong version":     "$argon2id$v=18$m=64,t=1,p=1$c2FsdA$aGFzaA",
		"bad params":        "$argon2id$v=19$m=x,t=1,p=1$c2FsdA$aGFzaA",
		"bad salt encoding": "$argon2id$v=19$m=64,t=1,p=1$!!!!$aGFzaA",
		"bad hash encoding": "$argon2id$v=19$m=64,t=1,p=1$c2FsdA$!!!!",
	}
	for name, encoded := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := h.Verify(encoded, "password"); err == nil {
				t.Fatalf("Verify(%q) error = nil, want an error", encoded)
			}
		})
	}
}

func TestHashPassword_RejectsEmptyPassword(t *testing.T) {
	t.Parallel()

	if _, err := testHasher().Hash(""); err == nil {
		t.Fatal("Hash(\"\") error = nil, want an error")
	}
}

func TestValidateNewPassword_U2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		confirm  string
		wantErr  error
	}{
		{name: "valid", password: "12345678", confirm: "12345678"},
		{name: "seven characters", password: "1234567", confirm: "1234567", wantErr: ErrPasswordTooShort},
		// #nosec G101 -- test fixture, not a real credential.
		{name: "eight runes", password: "åäöåäöåä", confirm: "åäöåäöåä"},
		{name: "mismatch", password: "12345678", confirm: "12345679", wantErr: ErrPasswordMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateNewPassword(tt.password, tt.confirm)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateNewPassword() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateNewPassword() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
