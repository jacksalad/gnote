package format

import "testing"

func TestFormatHTML_FormatsInlineStyleAndScript(t *testing.T) {
	src := `<html><head><style>a{color:red;margin:0}</style><script>const x=1;function f(){return x}</script></head><body></body></html>
`
	want := `<html>
    <head>
        <style>
            a {
                color: red;
                margin: 0;
            }
        </style>
        <script>
            const x = 1;
            function f() {
                return x
            }
        </script>
    </head>
    <body></body>
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

func TestFormatHTML_SkipsNonJSScript(t *testing.T) {
	src := `<div><script type="application/json">{"a":1}</script></div>`
	want := "<div>\n    <script type=\"application/json\">{\"a\":1}</script>\n</div>"
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("non-JS script was modified:\n got %q\nwant %q", got, want)
	}
}

func TestFormatHTML_InvalidScriptFallsBackToRaw(t *testing.T) {
	src := `<div><script>function f({</script></div>`
	want := "<div>\n    <script>function f({</script>\n</div>"
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("invalid script should be preserved:\n got %q\nwant %q", got, want)
	}
}

func TestFormatHTML_MultilineTemplateInScriptIsNotReindented(t *testing.T) {
	src := "<div><script>const t = `a\nb`;</script></div>"
	want := "<div>\n    <script>\n        const t = `a\nb`;\n    </script>\n</div>"
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("template literal was reindented:\n got %q\nwant %q", got, want)
	}
}

func TestFormatHTML_StyleURLPreserved(t *testing.T) {
	src := `<style>.a{background:url(data:image/png;base64,AAA)}</style>`
	want := `<style>
    .a {
        background: url(data:image/png;base64,AAA);
    }
</style>`
	got, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
