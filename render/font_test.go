package render_test

import (
	"os"
	"testing"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
)

func TestCanvasFontEngine(t *testing.T) {
	c := render.NewCanvas(200, 60)
	c.Clear(graphics.ColorWhite)

	font := graphics.NewFont("Microsoft YaHei", 16)
	testStr := "你好，世界！goui"

	w, h := c.MeasureText(testStr, font)
	t.Logf("Measured '%s': %dx%d", testStr, w, h)
	if w <= 0 || h <= 0 {
		t.Fatalf("MeasureText returned invalid dimensions: %dx%d", w, h)
	}

	c.DrawText(testStr, 10, 10, font, graphics.ColorBlack)

	if !c.IsDirty() {
		t.Fatalf("Canvas should be dirty after DrawText")
	}
}

func TestCanvasMultilineFontEngine(t *testing.T) {
	c := render.NewCanvas(300, 150)
	c.Clear(graphics.ColorWhite)

	font := graphics.NewFont("Microsoft YaHei", 14)
	singleLine := "测试行"
	multiline := "第一行\n第二行\r\n第三行\t制表"

	_, h1 := c.MeasureText(singleLine, font)
	wM, hM := c.MeasureText(multiline, font)

	if hM < h1*3 {
		t.Fatalf("expected multiline height >= 3 * singleline height, got hM=%d, h1=%d", hM, h1)
	}
	if wM <= 0 {
		t.Fatalf("expected multiline width > 0, got %d", wM)
	}

	// Ensure DrawText with newlines does not panic or crash
	c.DrawText(multiline, 10, 10, font, graphics.ColorBlack)
}

func TestRegisterMemoryFont(t *testing.T) {
	data, err := os.ReadFile("../examples/notepad/font.ttf")
	if err != nil {
		t.Skipf("skipping TestRegisterMemoryFont, font.ttf not found: %v", err)
	}

	family, err := render.ExtractFontFamily(data)
	if err != nil {
		t.Fatalf("ExtractFontFamily failed: %v", err)
	}
	if family != "yyb" {
		t.Errorf("expected family 'yyb', got '%s'", family)
	}

	registeredFamily, err := render.RegisterMemoryFont(data)
	if err != nil {
		t.Fatalf("RegisterMemoryFont failed: %v", err)
	}
	if registeredFamily != family {
		t.Errorf("expected registered family '%s', got '%s'", family, registeredFamily)
	}

	// 验证在画布上以新注册的字体渲染与量算
	c := render.NewCanvas(400, 100)
	c.Clear(graphics.ColorWhite)
	font := graphics.NewFont(registeredFamily, 16)

	testStr := "goui 记事本内嵌字体测试 123 !@#"
	w, h := c.MeasureText(testStr, font)
	if w <= 0 || h <= 0 {
		t.Fatalf("MeasureText on registered memory font failed: %dx%d", w, h)
	}

	c.DrawText(testStr, 10, 10, font, graphics.ColorBlack)
	if !c.IsDirty() {
		t.Errorf("canvas should be marked dirty after DrawText")
	}
}
