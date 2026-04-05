package mcp

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftwerx/company-research-mcp/internal/cache"
)

// errReader is a reader that immediately returns an error, used to test read failure paths.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// buildZip creates an in-memory zip archive from a slice of [name, content] pairs.
func buildZip(t *testing.T, files [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.Create(f[0])
		require.NoError(t, err, "buildZip: create %q", f[0])
		_, err = fw.Write([]byte(f[1]))
		require.NoError(t, err, "buildZip: write %q", f[0])
	}
	require.NoError(t, w.Close(), "buildZip: close")
	return buf.Bytes()
}

func TestExtractFromZip(t *testing.T) {
	t.Parallel()

	const limit = 1024 * 1024 // 1 MiB — small limit for tests

	cases := []struct {
		name            string
		files           [][2]string
		wantFilename    string
		wantContentType string
		wantContent     string
		wantErr         bool
	}{
		{
			name:            "should return the primary xhtml file",
			files:           [][2]string{{"report.xhtml", "<html/>"}},
			wantFilename:    "report.xhtml",
			wantContentType: "application/xhtml+xml",
			wantContent:     "<html/>",
		},
		{
			name: "should prefer xhtml over pdf when both present",
			files: [][2]string{
				{"filing.pdf", "PDF content"},
				{"report.xhtml", "<html/>"},
			},
			wantFilename:    "report.xhtml",
			wantContentType: "application/xhtml+xml",
			wantContent:     "<html/>",
		},
		{
			name: "should select largest when multiple xhtml files",
			files: [][2]string{
				{"small.xhtml", "<s/>"},
				{"large.xhtml", "<large>lots of content here</large>"},
			},
			wantFilename:    "large.xhtml",
			wantContentType: "application/xhtml+xml",
			wantContent:     "<large>lots of content here</large>",
		},
		{
			name: "should prefer html over pdf",
			files: [][2]string{
				{"filing.pdf", "PDF content"},
				{"report.html", "<html/>"},
			},
			wantFilename:    "report.html",
			wantContentType: "text/html",
			wantContent:     "<html/>",
		},
		{
			name:            "should fall back to largest file when no recognised extension",
			files:           [][2]string{{"data.bin", "binary content"}},
			wantFilename:    "data.bin",
			wantContentType: "application/octet-stream",
			wantContent:     "binary content",
		},
		{
			name:    "should return error for empty zip",
			files:   nil,
			wantErr: true,
		},
		{
			name:            "should return base filename not full zip path",
			files:           [][2]string{{"dir/subdir/report-2024-T01.xhtml", "<html/>"}},
			wantFilename:    "report-2024-T01.xhtml",
			wantContentType: "application/xhtml+xml",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			zipData := buildZip(t, test.files)

			// Act
			content, filename, contentType, err := extractFromZip(zipData, limit)

			// Assert
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantFilename, filename)
			if test.wantContentType != "" {
				assert.Equal(t, test.wantContentType, contentType)
			}
			if test.wantContent != "" {
				assert.Equal(t, test.wantContent, string(content))
			}
		})
	}
}

func TestExtractFromZip_ZipBomb(t *testing.T) {
	t.Parallel()

	// Arrange — use a real payload against a tiny limit to exercise the size-check path.
	const tinyLimit = 10 // bytes
	zipData := buildZip(t, [][2]string{{"report.xhtml", "more than ten bytes of content here"}})

	// Act
	_, _, _, err := extractFromZip(zipData, tinyLimit)

	// Assert
	assert.ErrorContains(t, err, "exceeds")
}

func TestExtractFromZip_Malformed(t *testing.T) {
	t.Parallel()

	// Arrange — not a valid zip
	notAZip := []byte("PK\x03\x04this is not a real zip archive")

	// Act
	_, _, _, err := extractFromZip(notAZip, cache.MaxFileSizeBytes)

	// Assert
	assert.ErrorContains(t, err, "open zip")
}

func TestReadZipBody(t *testing.T) {
	t.Parallel()

	t.Run("should return data when body is within limit", func(t *testing.T) {
		t.Parallel()

		// Arrange
		const limit = 100
		body := strings.NewReader("small content")

		// Act
		data, err := readZipBody(body, limit)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "small content", string(data))
	})

	t.Run("should return errZipTooLarge when body exceeds limit", func(t *testing.T) {
		t.Parallel()

		// Arrange
		const limit = 10
		oversized := bytes.Repeat([]byte{'X'}, int(limit)+5)

		// Act
		_, err := readZipBody(bytes.NewReader(oversized), limit)

		// Assert
		assert.True(t, errors.Is(err, errZipTooLarge))
	})

	t.Run("should return a wrapped error when the reader fails", func(t *testing.T) {
		t.Parallel()

		// Act
		_, err := readZipBody(errReader{}, cache.MaxFileSizeBytes)

		// Assert
		assert.ErrorContains(t, err, "read zip body")
	})
}
