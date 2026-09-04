package profile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"just-db/internal/engine"
)

const (
	dirName     = "profiles"
	keyFileName = "key"
	fileExt     = ".jdb"
	magic       = "JDB1"
	envKey      = "JUSTDB_PROFILES_KEY"
)

var (
	ErrNotFound    = errors.New("profile not found")
	ErrInvalidName = errors.New("invalid profile name")
	ErrDecrypt     = errors.New("cannot decrypt profile")

	namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]{0,63}$`)
)

// Record is a named engine + connection. Get returns the password.
// List returns Summary values with the password stripped.
type Record struct {
	Name       string            `json:"name"`
	Engine     engine.Name       `json:"engine"`
	Connection engine.Connection `json:"connection"`
}

// Summary is what List returns: enough to pick a profile, no secret.
type Summary struct {
	Name     string      `json:"name"`
	Engine   engine.Name `json:"engine"`
	Host     string      `json:"host"`
	Port     int         `json:"port"`
	User     string      `json:"user"`
	Database string      `json:"database"`
	SSLMode  string      `json:"sslMode"`
}

func (r Record) Summary() Summary {
	return Summary{
		Name:     r.Name,
		Engine:   r.Engine,
		Host:     r.Connection.Host,
		Port:     r.Connection.Port,
		User:     r.Connection.User,
		Database: r.Connection.Database,
		SSLMode:  r.Connection.SSLMode,
	}
}

func Dir(dataDir string) string {
	return filepath.Join(dataDir, dirName)
}

// Store keeps profiles as AES-256-GCM files under {dataDir}/profiles.
// The key comes from JUSTDB_PROFILES_KEY, or a 0600 key file created on first Open.
type Store struct {
	dir string
	key []byte
}

func Open(dataDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	dir := Dir(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	key, err := loadOrCreateKey(dir)
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, key: key}, nil
}

func (s *Store) List() ([]Summary, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]Summary, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != fileExt {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), fileExt)
		rec, err := s.Get(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, rec.Summary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) Get(name string) (Record, error) {
	if err := ValidateName(name); err != nil {
		return Record{}, err
	}
	raw, err := os.ReadFile(s.path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Record{}, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return Record{}, err
	}
	plain, err := decrypt(s.key, raw)
	if err != nil {
		return Record{}, err
	}
	var rec Record
	if err := json.Unmarshal(plain, &rec); err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	rec.Name = name
	return rec, nil
}

func (s *Store) Put(rec Record) error {
	rec.Name = strings.TrimSpace(rec.Name)
	if err := ValidateName(rec.Name); err != nil {
		return err
	}
	if rec.Engine == "" {
		return fmt.Errorf("%w: engine is required", engine.ErrInvalidConnection)
	}
	if err := rec.Connection.Validate(); err != nil {
		return err
	}
	plain, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	blob, err := encrypt(s.key, plain)
	if err != nil {
		return err
	}
	return writePrivate(s.path(rec.Name), blob)
}

func (s *Store) Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	err := os.Remove(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return err
}

func (s *Store) path(name string) string {
	return filepath.Join(s.dir, name+fileExt)
}

func ValidateName(name string) error {
	if !namePattern.MatchString(name) || name == keyFileName {
		return fmt.Errorf("%w: %q (use letters, digits, space, dot, underscore, hyphen; 1-64 chars)", ErrInvalidName, name)
	}
	return nil
}

func loadOrCreateKey(dir string) ([]byte, error) {
	if raw := strings.TrimSpace(os.Getenv(envKey)); raw != "" {
		return deriveKey(raw), nil
	}
	path := filepath.Join(dir, keyFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("profile key file %s must be 32 bytes", path)
		}
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := writePrivate(path, key); err != nil {
		return nil, err
	}
	return key, nil
}

func deriveKey(secret string) []byte {
	if decoded, err := hex.DecodeString(secret); err == nil && len(decoded) == 32 {
		return decoded
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func encrypt(key, plaintext []byte) ([]byte, error) {
	gcm, err := aead(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ad := []byte(magic)
	out := make([]byte, 0, len(ad)+len(nonce)+len(plaintext)+gcm.Overhead())
	out = append(out, ad...)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plaintext, ad), nil
}

func decrypt(key, blob []byte) ([]byte, error) {
	ad := []byte(magic)
	gcm, err := aead(key)
	if err != nil {
		return nil, err
	}
	need := len(ad) + gcm.NonceSize()
	if len(blob) < need {
		return nil, ErrDecrypt
	}
	if string(blob[:len(ad)]) != magic {
		return nil, ErrDecrypt
	}
	nonce := blob[len(ad):need]
	plain, err := gcm.Open(nil, nonce, blob[need:], ad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plain, nil
}

func aead(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func writePrivate(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}
