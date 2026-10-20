package core_auth_password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// These are application limits, checked before allocating Argon2's working memory.
const (
	minMemory            = 19 * 1024 // KiB
	maxMemory            = 64 * 1024
	minIterations        = 2
	maxIterations        = 4
	maxParallelism       = 4
	minSaltLength        = 16
	maxSaltLength        = 64
	minKeyLength         = 16
	maxKeyLength         = 64
	maxEncodedHashLength = 512
)

type Config struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32 // bytes
	KeyLength   uint32 // bytes
}

func DefaultConfig() Config {
	return Config{
		Memory:      minMemory,
		Iterations:  minIterations,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

type Hasher struct {
	config Config
}

func NewHasher(config Config) (*Hasher, error) {
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("password hasher config: %w", err)
	}
	return &Hasher{config: config}, nil
}

func (c Config) validate() error {
	if c.Memory < minMemory || c.Memory > maxMemory {
		return fmt.Errorf("memory must be between %d and %d KiB", minMemory, maxMemory)
	}
	if c.Iterations < minIterations || c.Iterations > maxIterations {
		return fmt.Errorf("iterations must be between %d and %d", minIterations, maxIterations)
	}
	if c.Parallelism < 1 || c.Parallelism > maxParallelism {
		return fmt.Errorf("parallelism must be between 1 and %d", maxParallelism)
	}
	if c.SaltLength < minSaltLength || c.SaltLength > maxSaltLength {
		return fmt.Errorf("salt length must be between %d and %d bytes", minSaltLength, maxSaltLength)
	}
	if c.KeyLength < minKeyLength || c.KeyLength > maxKeyLength {
		return fmt.Errorf("hash length must be between %d and %d bytes", minKeyLength, maxKeyLength)
	}
	return nil
}

func (h *Hasher) Hash(password string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("password hasher is nil")
	}
	if err := h.config.validate(); err != nil {
		return "", fmt.Errorf("password hasher config: %w", err)
	}
	c := h.config
	salt := make([]byte, c.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, c.Iterations, c.Memory, c.Parallelism, c.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, c.Memory, c.Iterations, c.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func (h *Hasher) Verify(password, encodedHash string) (bool, error) {
	c, salt, expected, err := parseHash(encodedHash)
	if err != nil {
		return false, fmt.Errorf("parse password hash: %w", err)
	}
	actual := argon2.IDKey([]byte(password), salt, c.Iterations, c.Memory, c.Parallelism, c.KeyLength)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func parseHash(encodedHash string) (Config, []byte, []byte, error) {
	if len(encodedHash) > maxEncodedHashLength {
		return Config{}, nil, nil, fmt.Errorf("encoded hash is too long")
	}
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" {
		return Config{}, nil, nil, fmt.Errorf("invalid hash format")
	}
	if parts[1] != "argon2id" {
		return Config{}, nil, nil, fmt.Errorf("unsupported hash algorithm")
	}
	if parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return Config{}, nil, nil, fmt.Errorf("unsupported Argon2 version")
	}

	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return Config{}, nil, nil, fmt.Errorf("expected memory, iterations and parallelism")
	}
	memory, err := parseParameter(params[0], "m=", 32)
	if err != nil {
		return Config{}, nil, nil, err
	}
	iterations, err := parseParameter(params[1], "t=", 32)
	if err != nil {
		return Config{}, nil, nil, err
	}
	parallelism, err := parseParameter(params[2], "p=", 8)
	if err != nil {
		return Config{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return Config{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}
	hash, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return Config{}, nil, nil, fmt.Errorf("decode hash: %w", err)
	}
	c := Config{
		Memory: uint32(memory), Iterations: uint32(iterations), Parallelism: uint8(parallelism),
		SaltLength: uint32(len(salt)), KeyLength: uint32(len(hash)),
	}
	if err := c.validate(); err != nil {
		return Config{}, nil, nil, err
	}
	return c, salt, hash, nil
}

func parseParameter(value, prefix string, bits int) (uint64, error) {
	raw, ok := strings.CutPrefix(value, prefix)
	if !ok || raw == "" {
		return 0, fmt.Errorf("invalid %s parameter", strings.TrimSuffix(prefix, "="))
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid %s parameter", strings.TrimSuffix(prefix, "="))
		}
	}
	number, err := strconv.ParseUint(raw, 10, bits)
	if err != nil {
		return 0, fmt.Errorf("parse %s parameter: %w", strings.TrimSuffix(prefix, "="), err)
	}
	return number, nil
}
