package core_auth_password_test

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	password "github.com/poponyas/TicketRush/internal/core/security/password"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerify(t *testing.T) {
	h, err := password.NewHasher(password.DefaultConfig())
	require.NoError(t, err)
	const secret = " Пароль с пробелами и Unicode 🔐 "
	first, err := h.Hash(secret)
	require.NoError(t, err)
	second, err := h.Hash(secret)
	require.NoError(t, err)
	require.NotEqual(t, first, second, "each hash must use a fresh salt")
	for _, encoded := range []string{first, second} {
		ok, err := h.Verify(secret, encoded)
		require.NoError(t, err)
		require.True(t, ok)
	}
	for _, wrong := range []string{"wrong", "", strings.TrimSpace(secret)} {
		ok, err := h.Verify(wrong, first)
		require.NoError(t, err)
		require.False(t, ok)
	}
}

func TestVerifyUsesStoredParameters(t *testing.T) {
	old := password.DefaultConfig()
	old.Memory = 32 * 1024
	old.Iterations = 3
	old.Parallelism = 2
	old.SaltLength = 24
	old.KeyLength = 48
	previous, err := password.NewHasher(old)
	require.NoError(t, err)
	encoded, err := previous.Hash("my password")
	require.NoError(t, err)
	current, err := password.NewHasher(password.DefaultConfig())
	require.NoError(t, err)
	ok, err := current.Verify("my password", encoded)
	require.NoError(t, err)
	require.True(t, ok, "changing defaults must not break existing hashes")
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	h, err := password.NewHasher(password.DefaultConfig())
	require.NoError(t, err)
	b64 := func(n int) string { return base64.RawStdEncoding.EncodeToString(make([]byte, n)) }
	format := func(params, salt, hash string) string {
		return fmt.Sprintf("$argon2id$v=19$%s$%s$%s", params, salt, hash)
	}
	valid := format("m=19456,t=2,p=1", b64(16), b64(32))
	cases := []string{
		"", "invalid", "$", "$argon2id", "argon2id$v=19$m=19456,t=2,p=1$salt$hash", valid + "$extra",
		strings.Replace(valid, "argon2id", "argon2i", 1), strings.Replace(valid, "v=19", "v=16", 1),
		format("m=19456", b64(16), b64(32)), format("m=19456,t=2", b64(16), b64(32)),
		format("m=19456,t=2,p=1,x=2", b64(16), b64(32)),
		format("m=19456,t=2,p=1", "***", b64(32)), format("m=19456,t=2,p=1", b64(16), "***"),
		format("m=19456,t=2,p=1", "", b64(32)), format("m=19456,t=2,p=1", b64(16), ""),
		format("m=19456,t=2,p=1", b64(15), b64(32)), format("m=19456,t=2,p=1", b64(65), b64(32)),
		format("m=19456,t=2,p=1", b64(16), b64(15)), format("m=19456,t=2,p=1", b64(16), b64(65)),
		strings.Repeat("x", 513),
	}
	for _, params := range []string{
		"m=0,t=2,p=1", "m=19455,t=2,p=1", "m=65537,t=2,p=1", "m=4294967296,t=2,p=1",
		"m=19456,t=0,p=1", "m=19456,t=5,p=1", "m=19456,t=4294967296,p=1",
		"m=19456,t=2,p=0", "m=19456,t=2,p=5", "m=19456,t=2,p=256",
		"m=abc,t=2,p=1", "m=-1,t=2,p=1", "m=+19456,t=2,p=1", "m=,t=2,p=1",
		"t=2,m=19456,p=1", "m=19456,t=2,t=1", "m=19456,t=2,p=1junk",
	} {
		cases = append(cases, format(params, b64(16), b64(32)))
	}
	for i, encoded := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			ok, err := h.Verify("password", encoded)
			require.Error(t, err)
			require.False(t, ok)
		})
	}
}

func TestNewHasherRejectsInvalidConfig(t *testing.T) {
	for _, change := range []func(*password.Config){
		func(c *password.Config) { c.Memory = 0 },
		func(c *password.Config) { c.Memory = 65537 },
		func(c *password.Config) { c.Iterations = 0 },
		func(c *password.Config) { c.Iterations = 5 },
		func(c *password.Config) { c.Parallelism = 0 },
		func(c *password.Config) { c.Parallelism = 5 },
		func(c *password.Config) { c.SaltLength = 0 },
		func(c *password.Config) { c.SaltLength = 65 },
		func(c *password.Config) { c.KeyLength = 0 },
		func(c *password.Config) { c.KeyLength = 65 },
	} {
		c := password.DefaultConfig()
		change(&c)
		h, err := password.NewHasher(c)
		require.Error(t, err)
		require.Nil(t, h)
	}
	var zero password.Hasher
	_, err := zero.Hash("password")
	require.Error(t, err)
	var missing *password.Hasher
	_, err = missing.Hash("password")
	require.Error(t, err)
}
