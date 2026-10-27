package syntax

import (
	"path/filepath"
	"strings"
)

// DetectHighlighter 根据文件名或后缀自动推荐并创建对应的语法高亮器
// 支持:
//   - Go: .go
//   - Python: .py, .pyw
//   - C/C++: .c, .h, .cpp, .hpp, .cc, .cxx, .c++, .h++, .hh
//   - C#: .cs, .csx
//   - Java: .java
//   - Rust: .rs
//   - R: .r, .rhistory, .rprofile
//   - JavaScript: .js, .mjs, .cjs, .jsx
//   - TypeScript / TypoScript: .ts, .tsx, .mts, .cts, .typoscript
//   - HTML: .html, .htm
//   - CSS: .css
//   - SQL: .sql, .dsql, .cql
//   - Markdown: .md, .markdown
//   - JSON: .json
//   - YAML: .yaml, .yml
//   - TOML: .toml
//
// 若文件类型无法匹配则返回 nil
func DetectHighlighter(pathOrExt string) Highlighter {
	ext := strings.ToLower(filepath.Ext(pathOrExt))
	if ext == "" {
		if strings.HasPrefix(pathOrExt, ".") {
			ext = strings.ToLower(pathOrExt)
		} else if !strings.Contains(pathOrExt, "/") && !strings.Contains(pathOrExt, "\\") {
			ext = "." + strings.ToLower(pathOrExt)
		}
	}

	switch ext {
	case ".go":
		return NewGoHighlighter()
	case ".py", ".pyw":
		return NewPythonHighlighter()
	case ".c", ".h", ".cpp", ".hpp", ".cc", ".cxx", ".c++", ".h++", ".hh":
		return NewCHighlighter()
	case ".cs", ".csx":
		return NewCSharpHighlighter()
	case ".java":
		return NewJavaHighlighter()
	case ".rs":
		return NewRustHighlighter()
	case ".r", ".rhistory", ".rprofile":
		return NewRHighlighter()
	case ".js", ".mjs", ".cjs", ".jsx":
		return NewJSHighlighter()
	case ".ts", ".tsx", ".mts", ".cts", ".typoscript":
		return NewTypeScriptHighlighter()
	case ".html", ".htm":
		return NewHTMLHighlighter()
	case ".css":
		return NewCSSHighlighter()
	case ".sql", ".dsql", ".cql":
		return NewSQLHighlighter()
	case ".md", ".markdown":
		return NewMarkdownHighlighter()
	case ".json":
		return NewJSONHighlighter()
	case ".yaml", ".yml":
		return NewYAMLHighlighter()
	case ".toml":
		return NewTOMLHighlighter()
	default:
		return nil
	}
}

// GetLanguageName 返回高亮器实例对应的友好语言名称 (如 Go, Python, C/C++, C#, Java, Rust, R, JavaScript, TypeScript, HTML, CSS, SQL, Markdown, JSON, YAML, TOML, 纯文本)
func GetLanguageName(h Highlighter) string {
	if h == nil {
		return "纯文本"
	}
	switch h.(type) {
	case *GoHighlighter:
		return "Go"
	case *PythonHighlighter:
		return "Python"
	case *CHighlighter:
		return "C/C++"
	case *CSharpHighlighter:
		return "C#"
	case *JavaHighlighter:
		return "Java"
	case *RustHighlighter:
		return "Rust"
	case *RHighlighter:
		return "R"
	case *JSHighlighter:
		return "JavaScript"
	case *TypeScriptHighlighter:
		return "TypeScript"
	case *HTMLHighlighter:
		return "HTML"
	case *CSSHighlighter:
		return "CSS"
	case *SQLHighlighter:
		return "SQL"
	case *MarkdownHighlighter:
		return "Markdown"
	case *JSONHighlighter:
		return "JSON"
	case *YAMLHighlighter:
		return "YAML"
	case *TOMLHighlighter:
		return "TOML"
	default:
		return "自定义高亮"
	}
}
