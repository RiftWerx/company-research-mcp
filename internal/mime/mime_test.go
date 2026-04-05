package mime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		contentType string
		want        string
	}{
		{"application/pdf", ".pdf"},
		{"application/pdf; charset=utf-8", ".pdf"},
		{"application/xhtml+xml", ".xhtml"},
		{"text/html", ".html"},
		{"application/octet-stream", ".bin"},
		{"", ".bin"},
	}

	for _, test := range cases {
		t.Run(test.contentType, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, Ext(test.contentType))
		})
	}
}

func TestTypeFromExt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ext  string
		want string
	}{
		{".xhtml", "application/xhtml+xml"},
		{".html", "text/html"},
		{".htm", "text/html"},
		{".pdf", "application/pdf"},
		{".bin", "application/octet-stream"},
		{".unknown", "application/octet-stream"},
	}

	for _, test := range cases {
		t.Run(test.ext, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, TypeFromExt(test.ext))
		})
	}
}
