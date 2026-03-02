package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileStore stores and retrieves tokens via the filesystem.
// It is the fallback when no platform keyring is available.
type FileStore struct {
	basePath string
}

// NewFileStore creates a FileStore backed by the given base directory.
// If basePath is empty, it defaults to $XDG_CONFIG_HOME/toolwright/ (or
// ~/.config/toolwright/ when XDG_CONFIG_HOME is unset).
func NewFileStore(basePath string) *FileStore {
	return &FileStore{basePath: basePath}
}

// resolvedBasePath returns the effective base directory, resolving XDG/HOME
// defaults when basePath is empty.
func (fs *FileStore) resolvedBasePath() string {
	if fs.basePath != "" {
		return filepath.Clean(fs.basePath)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "toolwright")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to HOME env var if UserHomeDir fails.
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "toolwright")
}

// tokensFilePath returns the path to the tokens.json file.
func (fs *FileStore) tokensFilePath() string {
	return filepath.Join(fs.resolvedBasePath(), "tokens.json")
}

// readTokenFile reads and parses tokens.json. Returns an empty TokenFile if
// the file does not exist.
func (fs *FileStore) readTokenFile(path string) (TokenFile, error) {
	tf := TokenFile{
		Version: 1,
		Tokens:  make(map[string]StoredToken),
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return tf, nil
	}
	if err != nil {
		return tf, fmt.Errorf("reading token file: %w", err)
	}

	if err := json.Unmarshal(data, &tf); err != nil {
		return tf, fmt.Errorf("parsing token file: %w", err)
	}
	if tf.Tokens == nil {
		tf.Tokens = make(map[string]StoredToken)
	}
	return tf, nil
}

// writeTokenFile serialises tf and writes it to path with 0600 permissions.
// It creates the directory tree if needed.
func writeTokenFile(path string, tf TokenFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}

	data, err := json.Marshal(tf)
	if err != nil {
		return fmt.Errorf("marshalling token file: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	return nil
}

// checkPermissions stats path and returns an error if the file permissions
// allow group or other access (i.e. mode & 0077 != 0).
func checkPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stating token file: %w", err)
	}
	if info.Mode().Perm() != 0600 {
		return fmt.Errorf("unsafe token file permission %04o: token file must have permission 0600 only; fix with: chmod 0600 %s",
			info.Mode().Perm(), path)
	}
	return nil
}

// Get retrieves a stored token from the file store.
// It opens the file once and checks permissions via Fstat on the open fd
// to avoid a TOCTOU race between permission check and read.
func (fs *FileStore) Get(key string) (*StoredToken, error) {
	path := fs.tokensFilePath()

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening token file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stating token file: %w", err)
	}
	if info.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("unsafe token file permission %04o: token file must have permission 0600 only; fix with: chmod 0600 %s",
			info.Mode().Perm(), path)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading token file: %w", err)
	}

	tf := TokenFile{
		Version: 1,
		Tokens:  make(map[string]StoredToken),
	}
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parsing token file: %w", err)
	}
	if tf.Tokens == nil {
		tf.Tokens = make(map[string]StoredToken)
	}

	tok, ok := tf.Tokens[key]
	if !ok {
		return nil, fmt.Errorf("token not found for key %q", key)
	}

	copy := tok
	return &copy, nil
}

// Set stores a token in the file store.
func (fs *FileStore) Set(key string, token StoredToken) error {
	path := fs.tokensFilePath()

	tf, err := fs.readTokenFile(path)
	if err != nil {
		return err
	}

	tf.Version = 1
	tf.Tokens[key] = token

	return writeTokenFile(path, tf)
}

// Delete removes a token from the file store.
func (fs *FileStore) Delete(key string) error {
	path := fs.tokensFilePath()

	tf, err := fs.readTokenFile(path)
	if err != nil {
		return err
	}

	// If the key does not exist, there is nothing to do.
	delete(tf.Tokens, key)

	return writeTokenFile(path, tf)
}
