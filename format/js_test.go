package format

import (
	"strings"
	"testing"
)

func TestFormatJS_BasicControlFlow(t *testing.T) {
	src := `function f(a,b){if(a){return 1}else{return 2}}`
	want := `function f(a, b) {
    if (a) {
        return 1
    } else {
        return 2
    }
}`
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_ForLoopKeepsHeaderOnOneLine(t *testing.T) {
	src := `for(let i=0;i<3;i++){console.log(i)}`
	want := `for (let i = 0; i < 3; i++) {
    console.log(i)
}`
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_StringsRegexAndTemplates(t *testing.T) {
	src := "const r=/a[/]b/g;const s=\"x};/\";const t=`a${b?1:2}c`;"
	want := "const r = /a[/]b/g;\nconst s = \"x};/\";\nconst t = `a${b?1:2}c`;"
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_DivisionVersusRegex(t *testing.T) {
	src := `const x=a/b;if(x)/re/.test(x);`
	want := "const x = a / b;\nif (x) /re/.test(x);"
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_PreservesASINewlineInsideCall(t *testing.T) {
	src := "foo(function(){return\n1});"
	want := "foo(function () {\n    return\n    1\n});"
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("nested ASI newline was removed:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_NumberDotSpacing(t *testing.T) {
	src := `const s=1 .toString();`
	want := `const s = 1 .toString();`
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("number-dot separation was lost:\n got %q\nwant %q", got, want)
	}
}

func TestFormatJS_DoWhileAndAsyncArrow(t *testing.T) {
	src := `do x();while(a);const f=async (x)=>x;`
	want := "do x();\nwhile (a);\nconst f = async (x) => x;"
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_Comments(t *testing.T) {
	src := "// hi\nconst a=1;/*x*/const b=2;\n/*\n * block\n */"
	want := "// hi\nconst a = 1;\n/*x*/ const b = 2;\n/*\n * block\n */"
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_PreservesASINewline(t *testing.T) {
	src := "function f(){return\n1}"
	want := "function f() {\n    return\n    1\n}"
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("ASI newline was removed:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_UnaryAndPostfixSpacing(t *testing.T) {
	src := `const x=-1;const y=a + +b;const z=i++ + j;`
	want := "const x = -1;\nconst y = a + +b;\nconst z = i++ + j;"
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_MultilineTemplateKeepsRawContent(t *testing.T) {
	got, err := formatJS("const t = `a\n  b`;", "    ")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	want := "    const t = `a\n  b`;"
	if got != want {
		t.Fatalf("template content was reindented:\n got %q\nwant %q", got, want)
	}
}

func TestFormatJS_RejectsUnbalancedInput(t *testing.T) {
	for _, src := range []string{"function f({", "}", "if (a) {"} {
		if _, err := formatJS(src, ""); err == nil {
			t.Fatalf("formatJS(%q) should fail", src)
		}
	}
}

func TestFormatJS_ObjectLiteralMultiline(t *testing.T) {
	src := `const o={a:1,b:{c:2}};`
	want := `const o = {
    a: 1,
    b: {
        c: 2
    }
};`
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected JS:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatJS_ElseCatchFinallyStayAttached(t *testing.T) {
	src := `try{f()}catch(e){g()}finally{h()}`
	got, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	for _, want := range []string{"} catch (e) {", "} finally {"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
