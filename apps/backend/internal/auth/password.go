package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/sync/semaphore"

	"phasma/backend/internal/env"
)

const (
	argonVersion = 19
	saltSize     = 16
	hashSize     = 32
)

var (
	ErrInvalidHash = errors.New("invalid password hash")
	ErrHashVersion = errors.New("unsupported argon2 version")
)

// NeedsRehash reports whether the stored hash was computed with parameters
// below the target — indicating it should be upgraded on the next login.
func NeedsRehash(encodedHash string, target PasswordParams) bool {
	params, _, _, err := parsePHC(encodedHash)
	if err != nil {
		return true
	}
	return params.Memory < target.Memory ||
		params.Iterations < target.Iterations ||
		params.Parallelism < target.Parallelism
}

// hashSemaphore caps concurrent Argon2 work so login bursts cannot exhaust pod
// memory; each default hash uses about 19 MiB.
var hashSemaphore = semaphore.NewWeighted(int64(env.Int("ARGON_MAX_CONCURRENCY", 4)))

type PasswordParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
}

var DefaultPasswordParams = PasswordParams{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
}

// decoyHash gives missing-account logins the same password-check cost, closing
// a user-enumeration timing oracle.
var decoyHash string

func init() {
	var err error
	decoyHash, err = HashPassword(context.Background(), "decoy password — never matches a real login", DefaultPasswordParams)
	if err != nil {
		panic(fmt.Sprintf("auth: failed to generate decoy hash: %v", err))
	}
}

// DecoyHash returns the process-wide decoy Argon2id hash.
func DecoyHash() string {
	return decoyHash
}

func HashPassword(ctx context.Context, password string, params PasswordParams) (string, error) {
	if params.Memory == 0 || params.Iterations == 0 || params.Parallelism == 0 {
		return "", ErrInvalidHash
	}

	if err := hashSemaphore.Acquire(ctx, 1); err != nil {
		return "", err
	}
	defer hashSemaphore.Release(1)

	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, hashSize)
	return encodePHC(params, salt, hash), nil
}

func VerifyPassword(ctx context.Context, password, encodedHash string) (bool, error) {
	params, salt, expected, err := parsePHC(encodedHash)
	if err != nil {
		return false, err
	}

	if err := hashSemaphore.Acquire(ctx, 1); err != nil {
		return false, err
	}
	defer hashSemaphore.Release(1)

	actual := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func encodePHC(params PasswordParams, salt, hash []byte) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersion,
		params.Memory,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

func parsePHC(encodedHash string) (PasswordParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	version, err := parseVersion(parts[2])
	if err != nil {
		return PasswordParams{}, nil, nil, err
	}
	if version != argonVersion {
		return PasswordParams{}, nil, nil, ErrHashVersion
	}

	params, err := parseParams(parts[3])
	if err != nil {
		return PasswordParams{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	if len(salt) == 0 || len(hash) == 0 {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	return params, salt, hash, nil
}

func parseVersion(value string) (int, error) {
	versionText, ok := strings.CutPrefix(value, "v=")
	if !ok {
		return 0, ErrInvalidHash
	}

	version, err := strconv.Atoi(versionText)
	if err != nil {
		return 0, ErrInvalidHash
	}
	return version, nil
}

func parseParams(value string) (PasswordParams, error) {
	values := map[string]uint64{}
	for _, part := range strings.Split(value, ",") {
		key, raw, ok := strings.Cut(part, "=")
		if !ok {
			return PasswordParams{}, ErrInvalidHash
		}
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return PasswordParams{}, ErrInvalidHash
		}
		values[key] = parsed
	}

	memory, hasMemory := values["m"]
	iterations, hasIterations := values["t"]
	parallelism, hasParallelism := values["p"]
	if !hasMemory || !hasIterations || !hasParallelism ||
		memory == 0 || iterations == 0 || parallelism == 0 || parallelism > 255 {
		return PasswordParams{}, ErrInvalidHash
	}

	return PasswordParams{
		Memory:      uint32(memory),
		Iterations:  uint32(iterations),
		Parallelism: uint8(parallelism),
	}, nil
}
