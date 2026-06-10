package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jr-k/d4s/internal/config"
	"golang.org/x/crypto/nacl/secretbox"
)

// Encrypted file store, used as fallback when no OS keyring is available
// (typically headless Linux without a Secret Service daemon).
// Secrets are encrypted with NaCl secretbox; the random key lives next to
// the data file, both with 0600 permissions.

const (
	keyFileName  = "credentials.key"
	dataFileName = "credentials.enc"
)

func fileStorePaths() (keyPath, dataPath string, err error) {
	dir := config.ConfigDir()
	if dir == "" {
		return "", "", fmt.Errorf("cannot resolve config directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	return filepath.Join(dir, keyFileName), filepath.Join(dir, dataFileName), nil
}

func loadOrCreateKey(keyPath string) (*[32]byte, error) {
	var key [32]byte

	if data, err := os.ReadFile(keyPath); err == nil {
		raw, err := hex.DecodeString(string(data))
		if err != nil || len(raw) != 32 {
			return nil, fmt.Errorf("corrupted credentials key file %s", keyPath)
		}
		copy(key[:], raw)
		return &key, nil
	}

	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key[:])), 0o600); err != nil {
		return nil, err
	}
	return &key, nil
}

func fileStoreReadAll() (map[string]SSHCredentials, error) {
	keyPath, dataPath, err := fileStorePaths()
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]SSHCredentials{}, nil
		}
		return nil, err
	}

	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}

	if len(raw) < 24 {
		return nil, fmt.Errorf("corrupted credentials file")
	}
	var nonce [24]byte
	copy(nonce[:], raw[:24])

	plain, ok := secretbox.Open(nil, raw[24:], &nonce, key)
	if !ok {
		return nil, fmt.Errorf("cannot decrypt credentials file (key mismatch?)")
	}

	all := map[string]SSHCredentials{}
	if err := json.Unmarshal(plain, &all); err != nil {
		return nil, err
	}
	return all, nil
}

func fileStoreWriteAll(all map[string]SSHCredentials) error {
	keyPath, dataPath, err := fileStorePaths()
	if err != nil {
		return err
	}

	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return err
	}

	plain, err := json.Marshal(all)
	if err != nil {
		return err
	}

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}

	out := secretbox.Seal(nonce[:], plain, &nonce, key)
	return os.WriteFile(dataPath, out, 0o600)
}

func fileStoreSave(contextName string, creds SSHCredentials) error {
	all, err := fileStoreReadAll()
	if err != nil {
		all = map[string]SSHCredentials{}
	}
	all[contextName] = creds
	return fileStoreWriteAll(all)
}

func fileStoreLoad(contextName string) (*SSHCredentials, error) {
	all, err := fileStoreReadAll()
	if err != nil {
		return nil, err
	}
	if creds, ok := all[contextName]; ok {
		return &creds, nil
	}
	return nil, nil
}

func fileStoreDelete(contextName string) {
	all, err := fileStoreReadAll()
	if err != nil {
		return
	}
	if _, ok := all[contextName]; !ok {
		return
	}
	delete(all, contextName)
	_ = fileStoreWriteAll(all)
}
