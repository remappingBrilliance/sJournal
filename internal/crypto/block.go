// Package crypto implements sJournal's self-contained encrypted block format:
// each block is base64(salt || nonce || ciphertext+tag), independently
// decryptable given the right password with no external metadata.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen  = 16
	nonceLen = 12
	keyLen   = 32 // AES-256

	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
)

var ErrDecryptFailed = errors.New("crypto: block could not be decrypted with this password")

// KeyCache derives and caches Argon2id keys so a given (password, salt) pair
// only pays the KDF cost once per process lifetime.
type KeyCache struct {
	mu   sync.Mutex
	keys map[string][]byte // key: password + "\x00" + string(salt)
}

func NewKeyCache() *KeyCache {
	return &KeyCache{keys: make(map[string][]byte)}
}

func (c *KeyCache) derive(password string, salt []byte) []byte {
	cacheKey := password + "\x00" + string(salt)

	c.mu.Lock()
	defer c.mu.Unlock()
	if k, ok := c.keys[cacheKey]; ok {
		return k
	}
	k := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keyLen)
	c.keys[cacheKey] = k
	return k
}

// sessionSalts remembers, per password, the salt chosen the first time that
// password was used to write a *new* block in this run, so repeated writes
// with the same password reuse one salt (and therefore one KDF derivation).
type sessionSalts struct {
	mu    sync.Mutex
	salts map[string][]byte
}

func newSessionSalts() *sessionSalts {
	return &sessionSalts{salts: make(map[string][]byte)}
}

func (s *sessionSalts) saltFor(password string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if salt, ok := s.salts[password]; ok {
		return salt, nil
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	s.salts[password] = salt
	return salt, nil
}

// Codec ties a KeyCache and a per-run salt cache together for encrypting and
// decrypting blocks with the self-contained on-disk format.
type Codec struct {
	keys  *KeyCache
	salts *sessionSalts
}

func NewCodec() *Codec {
	return &Codec{keys: NewKeyCache(), salts: newSessionSalts()}
}

// Encrypt encrypts plaintext with password, returning one base64 line ready
// to append to a note file. The salt used is the session's cached salt for
// this password (generated once, reused thereafter); the nonce is always
// freshly randomized.
func (c *Codec) Encrypt(password string, plaintext string) (string, error) {
	salt, err := c.salts.saltFor(password)
	if err != nil {
		return "", err
	}
	key := c.keys.derive(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	payload := make([]byte, 0, saltLen+nonceLen+len(ciphertext))
	payload = append(payload, salt...)
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)

	return base64.StdEncoding.EncodeToString(payload), nil
}

// Decrypt attempts to decrypt a stored block line with password. Returns
// ErrDecryptFailed if the line is malformed, too short, or the password is
// wrong (these are deliberately not distinguished — see spec).
func (c *Codec) Decrypt(password string, line string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return "", ErrDecryptFailed
	}
	if len(payload) < saltLen+nonceLen {
		return "", ErrDecryptFailed
	}

	salt := payload[:saltLen]
	nonce := payload[saltLen : saltLen+nonceLen]
	ciphertext := payload[saltLen+nonceLen:]

	key := c.keys.derive(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrDecryptFailed
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrDecryptFailed
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptFailed
	}
	return string(plaintext), nil
}
