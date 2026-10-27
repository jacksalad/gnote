package component

import (
	"testing"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

func TestCascadingThemePropagation(t *testing.T) {
	// Set initial dark theme
	theme.SetCurrent(theme.Dark())
	dark := theme.Dark()
	light := theme.Light()

	// 1. Build hierarchy
	root := NewBackgroundPanel()
	if root.background != dark.Background {
		t.Fatalf("expected root background %v, got %v", dark.Background, root.background)
	}

	card := NewSurfacePanel()
	if card.background != dark.Surface {
		t.Fatalf("expected card background %v, got %v", dark.Surface, card.background)
	}
	if card.borderColor != dark.Border {
		t.Fatalf("expected card border %v, got %v", dark.Border, card.borderColor)
	}

	btnNorm := NewButton("普通按钮")
	btnPrim := NewPrimaryButton("主要按钮")

	lblNorm := NewLabel("正文文本")
	lblSec := NewSecondaryLabel("次要说明")
	lblBrand := NewBrandLabel("品牌强调")

	tb := NewTextBox()
	cb := NewCheckBox("复选框")
	rb := NewRadioButton("单选框")

	card.Add(btnNorm)
	card.Add(btnPrim)
	card.Add(lblNorm)
	card.Add(lblSec)
	card.Add(lblBrand)
	card.Add(tb)
	card.Add(cb)
	card.Add(rb)

	sv := NewScrollView()
	innerPanel := NewPanelWithRole(PanelRoleSurfaceVariant)
	innerLabel := NewLabel("滚动条内部标签")
	innerPanel.Add(innerLabel)
	sv.SetContent(innerPanel)
	root.Add(sv)

	root.Add(card)

	// 2. Verify dark theme colors on components
	if btnNorm.normalBg != dark.SurfaceVariant {
		t.Errorf("expected btnNorm dark bg %v, got %v", dark.SurfaceVariant, btnNorm.normalBg)
	}
	if btnPrim.normalBg != dark.Primary {
		t.Errorf("expected btnPrim dark bg %v, got %v", dark.Primary, btnPrim.normalBg)
	}
	if lblNorm.textColor != dark.TextPrimary {
		t.Errorf("expected lblNorm dark text %v, got %v", dark.TextPrimary, lblNorm.textColor)
	}
	if lblSec.textColor != dark.TextSecondary {
		t.Errorf("expected lblSec dark text %v, got %v", dark.TextSecondary, lblSec.textColor)
	}
	if lblBrand.textColor != dark.Primary {
		t.Errorf("expected lblBrand dark text %v, got %v", dark.Primary, lblBrand.textColor)
	}
	if tb.background != dark.Surface {
		t.Errorf("expected tb dark bg %v, got %v", dark.Surface, tb.background)
	}
	if cb.textColor != dark.TextPrimary {
		t.Errorf("expected cb dark text %v, got %v", dark.TextPrimary, cb.textColor)
	}
	if rb.textColor != dark.TextPrimary {
		t.Errorf("expected rb dark text %v, got %v", dark.TextPrimary, rb.textColor)
	}
	if innerPanel.background != dark.SurfaceVariant {
		t.Errorf("expected innerPanel dark bg %v, got %v", dark.SurfaceVariant, innerPanel.background)
	}

	// 3. Trigger dynamic cascade to Light theme
	theme.SetCurrent(light)
	root.ApplyTheme(light)

	// Verify all components switched to Light theme
	if root.background != light.Background {
		t.Errorf("root didn't update to light background: expected %v, got %v", light.Background, root.background)
	}
	if card.background != light.Surface {
		t.Errorf("card didn't update to light surface: expected %v, got %v", light.Surface, card.background)
	}
	if card.borderColor != light.Border {
		t.Errorf("card didn't update to light border: expected %v, got %v", light.Border, card.borderColor)
	}
	if btnNorm.normalBg != light.SurfaceVariant {
		t.Errorf("btnNorm didn't update to light surface variant: expected %v, got %v", light.SurfaceVariant, btnNorm.normalBg)
	}
	if btnNorm.textColor != light.TextPrimary {
		t.Errorf("btnNorm didn't update to light text: expected %v, got %v", light.TextPrimary, btnNorm.textColor)
	}
	if btnPrim.normalBg != light.Primary {
		t.Errorf("btnPrim didn't update to light primary: expected %v, got %v", light.Primary, btnPrim.normalBg)
	}
	if lblNorm.textColor != light.TextPrimary {
		t.Errorf("lblNorm didn't update to light text primary: expected %v, got %v", light.TextPrimary, lblNorm.textColor)
	}
	if lblSec.textColor != light.TextSecondary {
		t.Errorf("lblSec didn't update to light text secondary: expected %v, got %v", light.TextSecondary, lblSec.textColor)
	}
	if lblBrand.textColor != light.Primary {
		t.Errorf("lblBrand didn't update to light primary: expected %v, got %v", light.Primary, lblBrand.textColor)
	}
	if tb.background != light.Surface {
		t.Errorf("tb didn't update to light surface: expected %v, got %v", light.Surface, tb.background)
	}
	if tb.textColor != light.TextPrimary {
		t.Errorf("tb didn't update to light text: expected %v, got %v", light.TextPrimary, tb.textColor)
	}
	if cb.textColor != light.TextPrimary {
		t.Errorf("cb didn't update to light text: expected %v, got %v", light.TextPrimary, cb.textColor)
	}
	if rb.textColor != light.TextPrimary {
		t.Errorf("rb didn't update to light text: expected %v, got %v", light.TextPrimary, rb.textColor)
	}
	if innerPanel.background != light.SurfaceVariant {
		t.Errorf("innerPanel didn't update to light surface variant: expected %v, got %v", light.SurfaceVariant, innerPanel.background)
	}
	if innerLabel.textColor != light.TextPrimary {
		t.Errorf("innerLabel didn't update to light text: expected %v, got %v", light.TextPrimary, innerLabel.textColor)
	}

	// 4. Test custom manual override preservation
	customBtn := NewButton("自定义按钮")
	customColor := graphics.RGB(123, 45, 67)
	customBtn.SetColors(customColor, customColor, customColor, customColor, graphics.ColorWhite, customColor)
	card.Add(customBtn)

	customLbl := NewLabel("自定义标签")
	customLbl.SetTextColor(customColor)
	card.Add(customLbl)

	// Switch back to Dark theme
	theme.SetCurrent(dark)
	root.ApplyTheme(dark)

	// Custom components must NOT have been overwritten
	if customBtn.normalBg != customColor {
		t.Errorf("custom button color was overwritten by theme: expected %v, got %v", customColor, customBtn.normalBg)
	}
	if customLbl.textColor != customColor {
		t.Errorf("custom label color was overwritten by theme: expected %v, got %v", customColor, customLbl.textColor)
	}
}
