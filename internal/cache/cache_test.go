package cache

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errReader is a reader that immediately returns an error, used to test Put error paths.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// infiniteReader is a reader that never returns EOF, used to test the Put size limit.
type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// newTestCache creates a Cache backed by a temp directory.
func newTestCache(t *testing.T) *Cache {
	t.Helper()
	c, err := New(Config{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("create test cache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestNewDefaultConfig(t *testing.T) {
	t.Parallel()

	// Act
	cfg := NewDefaultConfig()

	// Assert — zero value is the only contract: callers rely on New accepting it
	assert.Empty(t, cfg.BaseDir)
}

// TestNewDefaultBaseDir exercises the code path where BaseDir is empty and the
// cache falls back to os.UserCacheDir(). Not run in parallel because t.Setenv
// modifies a process-wide environment variable.
// XDG_CACHE_HOME controls os.UserCacheDir() on Linux; skipped on other platforms.
func TestNewDefaultBaseDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME only controls os.UserCacheDir on Linux")
	}

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	// Act
	c, err := New(Config{}) // empty BaseDir → resolves via XDG_CACHE_HOME

	// Assert
	assert.NoError(t, err)
	if c != nil {
		defer c.Close()
	}
	assert.DirExists(t, filepath.Join(tmpDir, "company-research.mcp"))
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("should create the database file on first open", func(t *testing.T) {
		t.Parallel()

		// Arrange
		dir := t.TempDir()

		// Act
		c, err := New(Config{BaseDir: dir})

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, c)
		defer c.Close()
		assert.FileExists(t, filepath.Join(dir, dbFileName))
	})

	t.Run("should return an error when the base directory cannot be created", func(t *testing.T) {
		t.Parallel()

		// Arrange: create a regular file, then try to use a path inside it as BaseDir.
		// os.MkdirAll cannot create a directory whose parent is a file.
		f, err := os.CreateTemp("", "not-a-dir-*")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Cleanup(func() { os.Remove(f.Name()) })

		// Act
		_, err = New(Config{BaseDir: filepath.Join(f.Name(), "subdir")})

		// Assert
		assert.Error(t, err)
	})
}

func TestPut(t *testing.T) {
	t.Parallel()

	t.Run("should store file on disk and return its local path", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)
		content := []byte("PDF filing content")

		// Act
		localPath, written, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader(content))

		// Assert
		assert.NoError(t, err)
		assert.EqualValues(t, len(content), written)
		assert.FileExists(t, localPath)
		assert.Equal(t, ".pdf", filepath.Ext(localPath))
		got, err := os.ReadFile(localPath)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("should return an error when the filing exceeds the size limit", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)

		// Act
		_, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", infiniteReader{})

		// Assert
		assert.ErrorContains(t, err, "size limit")
		// Verify no temp files were left behind.
		matches, _ := filepath.Glob(filepath.Join(c.baseDir, cacheSubDir, "00445790", "abc123", "*.tmp"))
		assert.Empty(t, matches, "temp file should be cleaned up on size limit error")
	})

	t.Run("should return an error and clean up the temp file when the reader fails", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)

		// Act
		_, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", errReader{})

		// Assert
		assert.Error(t, err)
		// Verify no temp files were left behind.
		matches, _ := filepath.Glob(filepath.Join(c.baseDir, cacheSubDir, "00445790", "abc123", "*.tmp"))
		assert.Empty(t, matches, "temp file should be cleaned up on reader error")
	})

	t.Run("should return an error and remove the committed file when the DB insert fails", func(t *testing.T) {
		t.Parallel()

		// Arrange: close the DB so the INSERT will fail.
		c := newTestCache(t)
		require.NoError(t, c.Close())

		// Act
		_, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader([]byte("data")))

		// Assert
		assert.Error(t, err)
		assert.NoFileExists(t, filepath.Join(c.baseDir, cacheSubDir, "00445790", "abc123", "filing.pdf"))
	})

	t.Run("should overwrite an existing entry", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)
		first := []byte("first")
		second := []byte("second, longer content")

		// Act
		_, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader(first))
		require.NoError(t, err)
		localPath, written, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader(second))

		// Assert
		assert.NoError(t, err)
		assert.EqualValues(t, len(second), written)
		got, err := os.ReadFile(localPath)
		require.NoError(t, err)
		assert.Equal(t, second, got)
	})
}

func TestGet(t *testing.T) {
	t.Parallel()

	t.Run("should return found=true for a stored document", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)
		content := []byte("filing data")
		storedPath, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader(content))
		require.NoError(t, err)

		// Act
		localPath, contentType, fileSize, found, err := c.Get(context.Background(), "00445790", "abc123")

		// Assert
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, storedPath, localPath)
		assert.Equal(t, "application/pdf", contentType)
		assert.EqualValues(t, len(content), fileSize)
	})

	t.Run("should return found=false when document is not in database", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)

		// Act
		_, _, _, found, err := c.Get(context.Background(), "00445790", "notexist")

		// Assert
		assert.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("should return found=false when file has been deleted from disk", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)
		storedPath, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader([]byte("data")))
		require.NoError(t, err)
		os.Remove(storedPath)

		// Act
		_, _, _, found, err := c.Get(context.Background(), "00445790", "abc123")

		// Assert
		assert.NoError(t, err)
		assert.False(t, found)
	})
}

func TestClear(t *testing.T) {
	t.Parallel()

	t.Run("should remove all files and DB rows when ch_number is empty", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)
		storedPath, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader([]byte("data1")))
		require.NoError(t, err)
		_, _, err = c.Put(context.Background(), "99999999", "def456", "application/pdf", bytes.NewReader([]byte("data2")))
		require.NoError(t, err)

		// Act
		deleted, freed, dbRecs, err := c.Clear(context.Background(), "")

		// Assert
		assert.NoError(t, err)
		assert.EqualValues(t, 2, deleted)
		assert.Greater(t, freed, int64(0))
		assert.EqualValues(t, 2, dbRecs)
		assert.NoFileExists(t, storedPath)
	})

	t.Run("should remove only the specified company's files and rows", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)
		_, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader([]byte("data1")))
		require.NoError(t, err)
		keptPath, _, err := c.Put(context.Background(), "99999999", "def456", "application/pdf", bytes.NewReader([]byte("data2")))
		require.NoError(t, err)

		// Act
		deleted, _, dbRecs, err := c.Clear(context.Background(), "00445790")

		// Assert
		assert.NoError(t, err)
		assert.EqualValues(t, 1, deleted)
		assert.EqualValues(t, 1, dbRecs)
		assert.FileExists(t, keptPath)
	})

	t.Run("should succeed when the cache is empty", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := newTestCache(t)

		// Act
		deleted, freed, dbRecs, err := c.Clear(context.Background(), "")

		// Assert
		assert.NoError(t, err)
		assert.EqualValues(t, 0, deleted)
		assert.EqualValues(t, 0, freed)
		assert.EqualValues(t, 0, dbRecs)
	})

	t.Run("should return an error when the DB delete fails", func(t *testing.T) {
		t.Parallel()

		// Arrange: close the DB so the DELETE statement will fail.
		// os.RemoveAll still succeeds (file system is fine).
		c := newTestCache(t)
		_, _, err := c.Put(context.Background(), "00445790", "abc123", "application/pdf", bytes.NewReader([]byte("data")))
		require.NoError(t, err)
		require.NoError(t, c.Close())

		// Act
		_, _, _, err = c.Clear(context.Background(), "")

		// Assert
		assert.Error(t, err)
	})
}

func TestFileExt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		contentType string
		want        string
	}{
		{"application/pdf", ".pdf"},
		{"application/pdf; charset=utf-8", ".pdf"},
		{"application/xhtml+xml", ".xhtml"},
		{"application/octet-stream", ".bin"},
		{"text/html", ".bin"},
		{"", ".bin"},
	}

	for _, test := range cases {
		t.Run(test.contentType, func(t *testing.T) {
			t.Parallel()

			// Act
			got := fileExt(test.contentType)

			// Assert
			assert.Equal(t, test.want, got)
		})
	}
}
