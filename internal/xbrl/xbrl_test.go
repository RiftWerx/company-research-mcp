package xbrl

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeXHTML writes content to a temporary .xhtml file and returns its path.
func writeXHTML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filing.xhtml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// minimalDoc wraps body into a minimal iXBRL document with the given contexts and units.
func minimalDoc(contexts, units, body string) string {
	return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8"/>
</head>
<body>
` + contexts + `
` + units + `
` + body + `
</body>
</html>`
}

const contextInstant = `
<xbrli:context id="ctx-instant">
  <xbrli:entity><xbrli:identifier scheme="http://www.companieshouse.gov.uk/">12345678</xbrli:identifier></xbrli:entity>
  <xbrli:period><xbrli:instant>2024-12-31</xbrli:instant></xbrli:period>
</xbrli:context>`

const contextDuration = `
<xbrli:context id="ctx-duration">
  <xbrli:entity><xbrli:identifier scheme="http://www.companieshouse.gov.uk/">12345678</xbrli:identifier></xbrli:entity>
  <xbrli:period>
    <xbrli:startDate>2024-01-01</xbrli:startDate>
    <xbrli:endDate>2024-12-31</xbrli:endDate>
  </xbrli:period>
</xbrli:context>`

const unitGBP = `
<xbrli:unit id="GBP">
  <xbrli:measure>iso4217:GBP</xbrli:measure>
</xbrli:unit>`

const unitPencePerShare = `
<xbrli:unit id="pps">
  <xbrli:divide>
    <xbrli:unitNumerator><xbrli:measure>iso4217:GBP</xbrli:measure></xbrli:unitNumerator>
    <xbrli:unitDenominator><xbrli:measure>xbrli:shares</xbrli:measure></xbrli:unitDenominator>
  </xbrli:divide>
</xbrli:unit>`

func TestParseXBRLFacts(t *testing.T) {
	t.Parallel()

	t.Run("should extract ix:nonFraction fact", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="uk-bus:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0">500000</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "Revenue", facts[0].Name)
		assert.InDelta(t, 500000.0, facts[0].Value, 0.001)
		assert.Equal(t, "2024-12-31", facts[0].Period)
		assert.Equal(t, "GBP", facts[0].Unit)
	})

	t.Run("should apply scale attribute to get actual value", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" scale="6" decimals="-1">181.7</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.InDelta(t, 181700000.0, facts[0].Value, 1.0)
	})

	t.Run("should apply negative scale to divide value", func(t *testing.T) {
		t.Parallel()

		// Arrange — scale="-3" means multiply by 10^-3 (i.e. divide by 1000)
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" scale="-3" decimals="0">5000</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.InDelta(t, 5.0, facts[0].Value, 0.0001)
	})

	t.Run("should apply sign='-' to negate value", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Loss" contextRef="ctx-instant" unitRef="GBP" sign="-" decimals="0">1000</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.InDelta(t, -1000.0, facts[0].Value, 0.001)
	})

	t.Run("should resolve contextRef to instant period", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Assets" contextRef="ctx-instant" unitRef="GBP" decimals="0">100</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "2024-12-31", facts[0].Period)
	})

	t.Run("should resolve contextRef to duration period in ISO 8601 interval format", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextDuration, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-duration" unitRef="GBP" decimals="0">100</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "2024-01-01/2024-12-31", facts[0].Period)
	})

	t.Run("should resolve unitRef to unit label stripping namespace prefix", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0">100</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "GBP", facts[0].Unit)
	})

	t.Run("should use numerator unit for divide units", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitPencePerShare,
			`<ix:nonFraction name="frs102:EPS" contextRef="ctx-instant" unitRef="pps" decimals="2">1.23</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "GBP", facts[0].Unit)
	})

	t.Run("should strip namespace prefix from concept name", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="uk-bus:UKCompaniesHouseRegisteredNumber" contextRef="ctx-instant" unitRef="GBP" decimals="0">1</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "UKCompaniesHouseRegisteredNumber", facts[0].Name)
	})

	t.Run("should filter facts by name_prefix", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP, `
			<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0">100</ix:nonFraction>
			<ix:nonFraction name="frs102:Profit" contextRef="ctx-instant" unitRef="GBP" decimals="0">50</ix:nonFraction>
		`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{NamePrefix: "Rev"})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "Revenue", facts[0].Name)
	})

	t.Run("should skip facts with nil attribute set to true", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" nil="true"></ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, facts)
	})

	t.Run("should concatenate span-split number text", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0"><span>1</span><span>2</span><span>3</span></ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.InDelta(t, 123.0, facts[0].Value, 0.001)
	})

	t.Run("should exclude text within ix:exclude subtrees", func(t *testing.T) {
		t.Parallel()

		// Arrange — ix:exclude contains display formatting that should not affect the parsed value.
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0">42<ix:exclude>(formatted)</ix:exclude></ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.InDelta(t, 42.0, facts[0].Value, 0.001)
	})

	t.Run("should omit ix:nonNumeric when include_text_facts is false", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, unitGBP, `
			<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0">100</ix:nonFraction>
			<ix:nonNumeric name="uk-bus:CompanyName" contextRef="ctx-instant">Acme Ltd</ix:nonNumeric>
		`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{IncludeTextFacts: false})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "Revenue", facts[0].Name)
	})

	t.Run("should include ix:nonNumeric text fact when include_text_facts is true", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, "",
			`<ix:nonNumeric name="uk-bus:CompanyName" contextRef="ctx-instant">Acme Ltd</ix:nonNumeric>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{IncludeTextFacts: true})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "CompanyName", facts[0].Name)
		assert.Equal(t, "Acme Ltd", facts[0].Value)
		assert.Equal(t, "2024-12-31", facts[0].Period)
		assert.Empty(t, facts[0].Unit)
	})

	t.Run("should filter text facts by name_prefix when include_text_facts is true", func(t *testing.T) {
		t.Parallel()

		// Arrange
		doc := minimalDoc(contextInstant, "", `
			<ix:nonNumeric name="uk-bus:CompanyName" contextRef="ctx-instant">Acme Ltd</ix:nonNumeric>
			<ix:nonNumeric name="uk-bus:DirectorName" contextRef="ctx-instant">Jane Smith</ix:nonNumeric>
		`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{IncludeTextFacts: true, NamePrefix: "Company"})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "CompanyName", facts[0].Name)
	})

	t.Run("should return empty slice for document with no iXBRL facts", func(t *testing.T) {
		t.Parallel()

		// Arrange
		path := writeXHTML(t, `<!DOCTYPE html><html><body><p>No XBRL here.</p></body></html>`)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, facts)
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		t.Parallel()

		// Arrange
		path := filepath.Join(t.TempDir(), "missing.xhtml")

		// Act
		_, _, err := ParseFacts(path, Options{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "open file")
	})

	t.Run("should return error when file exceeds MaxFileSizeBytes", func(t *testing.T) {
		t.Parallel()

		// Arrange — write a real file then use a stat-only approach via a fake size check.
		// We write a valid file but temporarily reduce the limit via an oversized stat check.
		// Since MaxFileSizeBytes is a package constant we cannot override, we use a file
		// whose size is 1 byte over MaxFileSizeBytes by writing sparse content.
		// Instead, we verify the error message using a direct helper call on a large-size file.
		// Create a file of exactly MaxFileSizeBytes+1 bytes.
		path := filepath.Join(t.TempDir(), "big.xhtml")
		f, createErr := os.Create(path)
		require.NoError(t, createErr)
		require.NoError(t, f.Truncate(MaxFileSizeBytes+1))
		require.NoError(t, f.Close())

		// Act
		_, _, err := ParseFacts(path, Options{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds limit")
	})

	t.Run("should return error when scale attribute is out of bounds", func(t *testing.T) {
		t.Parallel()

		// Arrange — scale of 100 is well outside [minScale, maxScale].
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" scale="100" decimals="0">1</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert — out-of-bounds scale causes the fact to be silently skipped (not a hard error).
		require.NoError(t, err)
		assert.Empty(t, facts)
	})

	t.Run("should cap output at MaxFacts", func(t *testing.T) {
		t.Parallel()

		// Arrange — generate MaxFacts+10 facts in a single document.
		var sb strings.Builder
		sb.WriteString(contextInstant)
		sb.WriteString(unitGBP)
		for i := range MaxFacts + 10 {
			sb.WriteString(`<ix:nonFraction name="frs102:Fact` + strconv.Itoa(i) + `" contextRef="ctx-instant" unitRef="GBP" decimals="0">1</ix:nonFraction>`)
		}
		path := writeXHTML(t, `<!DOCTYPE html><html><body>`+sb.String()+`</body></html>`)

		// Act
		facts, truncated, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		assert.Len(t, facts, MaxFacts)
		assert.True(t, truncated, "truncated should be true when document exceeds MaxFacts")
	})

	t.Run("should ignore link:schemaRef and other external reference elements", func(t *testing.T) {
		t.Parallel()

		// Arrange — schemaRef and linkbaseRef must not affect output or trigger fetching.
		doc := `<!DOCTYPE html>
<html>
<head>
  <link:schemaRef xlink:href="http://malicious.example.com/schema.xsd" xlink:type="simple"/>
</head>
<body>
` + contextInstant + unitGBP + `
<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0">99</ix:nonFraction>
</body>
</html>`
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "Revenue", facts[0].Name)
	})

	t.Run("should skip numeric fact when ParseFloat returns Inf", func(t *testing.T) {
		t.Parallel()

		// Arrange — "1e400" is a valid float string that ParseFloat returns as +Inf with no error.
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" decimals="0">1e400</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert — Inf cannot be marshaled to JSON; fact must be silently dropped.
		require.NoError(t, err)
		assert.Empty(t, facts)
	})

	t.Run("should skip numeric fact when scale causes overflow to Inf", func(t *testing.T) {
		t.Parallel()

		// Arrange — large but finite value × 10^15 overflows to +Inf.
		doc := minimalDoc(contextInstant, unitGBP,
			`<ix:nonFraction name="frs102:Revenue" contextRef="ctx-instant" unitRef="GBP" scale="15" decimals="0">1.8e308</ix:nonFraction>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, facts)
	})

	t.Run("should cap context map at maxContexts", func(t *testing.T) {
		t.Parallel()

		// Arrange — generate maxContexts+5 distinct contexts plus one fact.
		var sb strings.Builder
		for i := range maxContexts + 5 {
			sb.WriteString(`<xbrli:context id="ctx` + strconv.Itoa(i) + `">` +
				`<xbrli:entity><xbrli:identifier scheme="x">1</xbrli:identifier></xbrli:entity>` +
				`<xbrli:period><xbrli:instant>2024-12-31</xbrli:instant></xbrli:period>` +
				`</xbrli:context>`)
		}
		sb.WriteString(unitGBP)
		// This fact uses a context ID beyond the cap; its period will be unresolved.
		sb.WriteString(`<ix:nonFraction name="frs102:Revenue" contextRef="ctx` + strconv.Itoa(maxContexts+1) + `" unitRef="GBP" decimals="0">1</ix:nonFraction>`)
		path := writeXHTML(t, `<!DOCTYPE html><html><body>`+sb.String()+`</body></html>`)

		// Act
		facts, _, err := ParseFacts(path, Options{})

		// Assert — the fact is still returned; its period is just empty (context was not stored).
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Empty(t, facts[0].Period, "context beyond cap should produce empty period")
	})

	t.Run("should truncate text fact value longer than maxTextFactLen", func(t *testing.T) {
		t.Parallel()

		// Arrange — generate a text value well beyond the cap.
		longText := strings.Repeat("x", maxTextFactLen+100)
		doc := minimalDoc(contextInstant, "",
			`<ix:nonNumeric name="uk-bus:Note" contextRef="ctx-instant">`+longText+`</ix:nonNumeric>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{IncludeTextFacts: true})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		text, ok := facts[0].Value.(string)
		require.True(t, ok)
		assert.LessOrEqual(t, len(text), maxTextFactLen+len("…"), "truncated value must not exceed cap")
		assert.True(t, strings.HasSuffix(text, "…"), "truncated value must end with ellipsis")
	})

	t.Run("should truncate multi-byte UTF-8 text without splitting a character", func(t *testing.T) {
		t.Parallel()

		// Arrange — £ is a 2-byte UTF-8 sequence; a byte-index slice at maxTextFactLen
		// would cut inside it, producing invalid UTF-8.
		longText := strings.Repeat("£", maxTextFactLen+10)
		doc := minimalDoc(contextInstant, "",
			`<ix:nonNumeric name="uk-bus:Note" contextRef="ctx-instant">`+longText+`</ix:nonNumeric>`)
		path := writeXHTML(t, doc)

		// Act
		facts, _, err := ParseFacts(path, Options{IncludeTextFacts: true})

		// Assert
		require.NoError(t, err)
		require.Len(t, facts, 1)
		text, ok := facts[0].Value.(string)
		require.True(t, ok)
		assert.True(t, strings.HasSuffix(text, "…"), "truncated value must end with ellipsis")
		assert.LessOrEqual(t, len([]rune(text)), maxTextFactLen+1, "truncated rune count must not exceed cap (+ellipsis rune)")
	})
}
