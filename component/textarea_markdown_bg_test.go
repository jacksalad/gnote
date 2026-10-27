package component

import (
	"os"
	"testing"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

// lumaOf 近似感知亮度 (0-255)，用于比较字形与底色的明暗距离。
func lumaOf(c graphics.Color) int {
	return (299*int(c.R()) + 587*int(c.G()) + 114*int(c.B())) / 1000
}

// TestTextArea_MarkdownInlineCodeContrast 保证 Markdown 行内代码的半透明底色
// 不会覆盖在字形之上。绘制顺序一旦颠倒，底色会把正文向背景方向混合，八套主题
// 的行内代码都会明显发灰变淡。
func TestTextArea_MarkdownInlineCodeContrast(t *testing.T) {
	fam := ""
	if data, err := os.ReadFile("../cmd/gnote/font.ttf"); err == nil {
		name, err := render.RegisterMemoryFont(data)
		if err != nil {
			t.Fatal(err)
		}
		fam = name
	} else {
		fam = graphics.DefaultFont.Name
	}

	prev := theme.Current()
	defer theme.SetCurrent(prev)

	const text = "alpha `code` omega"

	ta := NewTextArea()
	ta.SetBounds(0, 0, 900, 200)
	if fam != "" {
		ta.SetFont(graphics.NewFont(fam, 16))
	}
	md := syntax.NewMarkdownHighlighter()
	ta.SetHighlighter(md)

	for _, th := range theme.All() {
		theme.SetCurrent(th)
		ta.ApplyTheme(th)
		ta.SetText(text)

		colors := md.GetColors()
		var codeSpan syntax.Span
		found := false
		for _, sp := range ta.getSpansForLine(0) {
			if sp.Color == colors.InlineCodeText && sp.BgColor == colors.InlineCodeBg {
				codeSpan = sp
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: 未找到行内代码 span", th.ID)
		}

		canvas := render.NewCanvas(900, 200)
		ta.Render(canvas)

		textStartX := ta.bounds.X + ta.getGutterWidth() + ta.margin
		lineY := ta.bounds.Y + ta.margin
		x1 := textStartX + ta.measureStyledVisualX(0, 0, codeSpan.StartCol)
		x2 := textStartX + ta.measureStyledVisualX(0, 0, codeSpan.EndCol)
		if x2 <= x1 {
			t.Fatalf("%s: 行内代码宽度非法 [%d,%d)", th.ID, x1, x2)
		}

		// 编辑器背景取自 span 左侧空白列，避开行内代码底色框。
		bgLuma := lumaOf(canvas.GetPixel(x1-4, lineY+ta.lineHeight/2))
		glyphLuma := lumaOf(colors.InlineCodeText)
		wantDist := glyphLuma - bgLuma
		if wantDist < 0 {
			wantDist = -wantDist
		}
		if wantDist < 40 {
			t.Fatalf("%s: 行内代码令牌对比度本身过低 (dist=%d)", th.ID, wantDist)
		}

		maxDist := 0
		for y := lineY; y < lineY+ta.lineHeight; y++ {
			for x := x1; x < x2; x++ {
				d := lumaOf(canvas.GetPixel(x, y)) - bgLuma
				if d < 0 {
					d = -d
				}
				if d > maxDist {
					maxDist = d
				}
			}
		}

		// 半透明底色压在字形上时最亮像素只剩约 45% 的明暗距离。
		if maxDist*4 < wantDist*3 {
			t.Fatalf("%s: 行内代码字形被底色冲淡，maxDist=%d wantDist=%d", th.ID, maxDist, wantDist)
		}
	}
}
