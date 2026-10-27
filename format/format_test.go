package format

import (
	"strings"
	"testing"
)

func TestKindForPath(t *testing.T) {
	cases := []struct {
		path string
		want Kind
	}{
		{"config.json", KindJSON},
		{"CONFIG.JSON", KindJSON},
		{`C:\docs\page.html`, KindHTML},
		{"index.HTM", KindHTML},
		{"notes.txt", KindNone},
		{"", KindNone},
		{"json", KindNone},
	}
	for _, tc := range cases {
		if got := KindForPath(tc.path); got != tc.want {
			t.Errorf("KindForPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFormatJSON_PreservesOrderNumbersAndNewline(t *testing.T) {
	src := "{\r\n\"b\":[1,2,{\"c\":null}],\"a\":1.2300,\"big\":123456789012345678901234567890,\"s\":\"\\u4e2d\"}\r\n"
	want := `{
    "b": [
        1,
        2,
        {
            "c": null
        }
    ],
    "a": 1.2300,
    "big": 123456789012345678901234567890,
    "s": "\u4e2d"
}
`
	got, err := FormatJSON(src)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected output:\n got %q\nwant %q", got, want)
	}
}

func TestFormatJSON_NoTrailingNewline(t *testing.T) {
	got, err := FormatJSON(`{"a":1}`)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if want := "{\n    \"a\": 1\n}"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatJSON_ReportsLineAndColumn(t *testing.T) {
	_, err := FormatJSON("{\n  \"a\": 1,\n  \"b\" 2\n}")
	if err == nil {
		t.Fatal("expected syntax error")
	}
	if !strings.Contains(err.Error(), "第 3 行") {
		t.Fatalf("error should mention line 3: %v", err)
	}
}

func TestFormatJSON_EmptyDocument(t *testing.T) {
	for _, src := range []string{"", " \r\n\t", "\uFEFF"} {
		if _, err := FormatJSON(src); err != ErrEmptyDocument {
			t.Fatalf("FormatJSON(%q) error = %v, want ErrEmptyDocument", src, err)
		}
	}
}

func TestFormatHTML_BlockNestingAndInlineContent(t *testing.T) {
	src := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>T</title></head><body><div class="a"><p>Hello <strong>world</strong>!</p><ul><li>One</li><li>Two</li></ul><img src="x.png"><script>if (a < b) { x(); }</script></div></body></html>
`
	want := `<!DOCTYPE html>
<html>
    <head>
        <meta charset="utf-8">
        <title>T</title>
    </head>
    <body>
        <div class="a">
            <p>Hello <strong>world</strong>!</p>
            <ul>
                <li>One</li>
                <li>Two</li>
            </ul>
            <img src="x.png">
            <script>
                if (a < b) {
                    x();
                }
            </script>
        </div>
    </body>
</html>
`
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatHTML_PreservesPreformattedContent(t *testing.T) {
	src := "<div>\n<pre>  a\n    b\n</pre>\n</div>\n"
	want := "<div>\n    <pre>  a\n    b\n    </pre>\n</div>\n"
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected output:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestFormatHTML_PreservesBlankLinesInPre(t *testing.T) {
	src := "<pre>a\n\n\tb</pre>"
	want := "<pre>a\n\n\tb</pre>"
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatHTML_AttributeWithGreaterThanAndQuotes(t *testing.T) {
	src := `<a title="a > b" data-x='c > d'>link</a>`
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != src {
		t.Fatalf("got %q, want %q", got, src)
	}
}

func TestFormatHTML_PreservesInlineWhitespace(t *testing.T) {
	src := `<p><b>a</b> <i>b</i> <a href="#">c</a></p>`
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != src {
		t.Fatalf("inline whitespace lost:\n got %q\nwant %q", got, src)
	}
}

func TestFormatHTML_PreservesSVGVerbatim(t *testing.T) {
	src := `<div><svg viewBox="0 0 10 10"><text> a  b </text></svg></div>`
	want := "<div>\n    <svg viewBox=\"0 0 10 10\"><text> a  b </text></svg>\n</div>"
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatHTML_SelectOptionsStayInsideSelect(t *testing.T) {
	src := `<select name="x"><option value="1">A<option value="2">B</select>`
	want := `<select name="x">
    <option value="1">A
    <option value="2">B
</select>`
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatHTML_OptionalEndTagsDoNotDriftIndent(t *testing.T) {
	src := `<ul><li>One<li>Two</ul>`
	want := "<ul>\n    <li>One\n    <li>Two\n</ul>"
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected output:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestFormatHTML_TableOptionalEndTagsStayAligned(t *testing.T) {
	src := `<table><tbody><tr><td>A<td>B<tr><td>C</tbody></table>`
	want := `<table>
    <tbody>
        <tr>
            <td>A
            <td>B
        <tr>
            <td>C
    </tbody>
</table>`
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatHTML_MalformedInputDoesNotPanic(t *testing.T) {
	for _, src := range []string{
		"<div><span>oops",
		"</div></span>",
		"<!-- unterminated",
		"a < b and c > d",
		"<script>",
		"<",
	} {
		if _, err := FormatHTML(src); err != nil {
			t.Fatalf("FormatHTML(%q): %v", src, err)
		}
	}
}

func TestFormatHTML_EmptyDocument(t *testing.T) {
	if _, err := FormatHTML(" \n\t"); err != ErrEmptyDocument {
		t.Fatalf("error = %v, want ErrEmptyDocument", err)
	}
}

func TestFormat_UnsupportedKind(t *testing.T) {
	if _, err := Format(KindNone, "x"); err != ErrUnsupported {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}
