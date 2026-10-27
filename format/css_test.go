package format

import (
	"strings"
	"testing"
)

func TestFormatCSS_BasicRulesAndMedia(t *testing.T) {
	src := `a,b{color:red;margin:0 auto}@media (min-width:600px){.x{display:none}}`
	want := `a,
b {
    color: red;
    margin: 0 auto;
}
@media (min-width:600px) {
    .x {
        display: none;
    }
}`
	got, err := formatCSS(src, "")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected CSS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatCSS_PreservesStringsURLsAndComments(t *testing.T) {
	src := `/* header */ .a{content:"a;b";background:url(data:image/png;base64,AAA)}`
	want := `/* header */
.a {
    content: "a;b";
    background: url(data:image/png;base64,AAA);
}`
	got, err := formatCSS(src, "")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected CSS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatCSS_ImportAndAtRules(t *testing.T) {
	src := `@charset "utf-8";@import url("a.css") screen;@keyframes fade{from{opacity:0}to{opacity:1}}`
	want := `@charset "utf-8";
@import url("a.css") screen;
@keyframes fade {
    from {
        opacity: 0;
    }
    to {
        opacity: 1;
    }
}`
	got, err := formatCSS(src, "")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected CSS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatCSS_BaseIndent(t *testing.T) {
	got, err := formatCSS(`a{color:red}`, "    ")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	want := "    a {\n        color: red;\n    }"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatCSS_RejectsUnbalancedStructure(t *testing.T) {
	for _, src := range []string{"a{color:red", "}", "a{color:red}}"} {
		if _, err := formatCSS(src, ""); err == nil {
			t.Fatalf("formatCSS(%q) should fail", src)
		}
	}
}

func TestFormatCSS_DoesNotSplitCommasInsideValues(t *testing.T) {
	src := `.a{font-family:"A",B;transform:translate(-50%,-50%)}`
	want := `.a {
    font-family: "A", B;
    transform: translate(-50%, -50%);
}`
	got, err := formatCSS(src, "")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected CSS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatCSS_URLWithBracesStaysLiteral(t *testing.T) {
	src := `.a{background:url(data:image/svg+xml,<svg>{}</svg>)}`
	want := `.a {
    background: url(data:image/svg+xml,<svg>{}</svg>);
}`
	got, err := formatCSS(src, "")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	if got != want {
		t.Fatalf("url content changed:\n got %q\nwant %q", got, want)
	}
}

func TestFormatCSS_SelectorPseudoSpacingPreserved(t *testing.T) {
	src := `a:hover,b :hover{color:red}`
	got, err := formatCSS(src, "")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	if !strings.Contains(got, "a:hover,") || !strings.Contains(got, "b :hover {") {
		t.Fatalf("pseudo spacing changed:\n%s", got)
	}
}
