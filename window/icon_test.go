//go:build windows

package window

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unsafe"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
)

func TestCreateHIconFromPixels(t *testing.T) {
	pixels := make([]graphics.Color, 16*16)
	for i := range pixels {
		pixels[i] = graphics.RGBA(0, 128, 255, 255)
	}

	hIcon, err := CreateHIconFromPixels(16, 16, pixels)
	if err != nil {
		t.Fatalf("CreateHIconFromPixels failed: %v", err)
	}
	if hIcon == 0 {
		t.Fatalf("expected non-zero hIcon")
	}
	defer DestroyIcon(hIcon)
}

func TestNewIconFromRGBA(t *testing.T) {
	smallPixels := make([]graphics.Color, 16*16)
	for i := range smallPixels {
		smallPixels[i] = graphics.RGBA(255, 100, 50, 200)
	}

	bigPixels := make([]graphics.Color, 32*32)
	for i := range bigPixels {
		bigPixels[i] = graphics.RGBA(50, 200, 100, 255)
	}

	icon, err := NewIconFromRGBA(16, 16, smallPixels, 32, 32, bigPixels)
	if err != nil {
		t.Fatalf("NewIconFromRGBA failed: %v", err)
	}
	defer icon.Destroy()

	if icon.GetSmallHandle() == 0 {
		t.Errorf("expected non-zero small icon handle")
	}
	if icon.GetBigHandle() == 0 {
		t.Errorf("expected non-zero big icon handle")
	}
}

func TestNewIconFromImage(t *testing.T) {
	img := render.NewImage(64, 64)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetPixel(int32(x), int32(y), graphics.RGBA(uint8(x*4), uint8(y*4), 180, 255))
		}
	}

	icon, err := NewIconFromImage(img)
	if err != nil {
		t.Fatalf("NewIconFromImage failed: %v", err)
	}
	defer icon.Destroy()

	if icon.GetSmallHandle() == 0 || icon.GetBigHandle() == 0 {
		t.Errorf("expected non-zero icon handles from image")
	}
}

func TestNewIconFromICO(t *testing.T) {
	// 构造一个合法的 16x16 32bpp ICO 格式二进制流
	w, h := 16, 16
	xorSize := w * h * 4
	andRowBytes := ((w + 31) / 32) * 4
	andSize := andRowBytes * h
	imgSize := 40 + xorSize + andSize

	buf := new(bytes.Buffer)
	// ICONDIR
	binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(buf, binary.LittleEndian, uint16(1)) // type: 1 = ico
	binary.Write(buf, binary.LittleEndian, uint16(1)) // count: 1

	// ICONDIRENTRY
	buf.WriteByte(byte(w))
	buf.WriteByte(byte(h))
	buf.WriteByte(0)                                   // color count
	buf.WriteByte(0)                                   // reserved
	binary.Write(buf, binary.LittleEndian, uint16(1))  // planes
	binary.Write(buf, binary.LittleEndian, uint16(32)) // bit count
	binary.Write(buf, binary.LittleEndian, uint32(imgSize))
	binary.Write(buf, binary.LittleEndian, uint32(6+16)) // offset

	// BITMAPINFOHEADER (height = h * 2)
	bmi := BITMAPINFOHEADER{
		BiSize:        40,
		BiWidth:       int32(w),
		BiHeight:      int32(h * 2),
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: 0,
		BiSizeImage:   uint32(xorSize + andSize),
	}
	bmiBytes := unsafe.Slice((*byte)(unsafe.Pointer(&bmi)), 40)
	buf.Write(bmiBytes)

	// XOR 像素数据 (全青色带全不透明 Alpha)
	xor := make([]byte, xorSize)
	for i := 0; i < w*h; i++ {
		xor[i*4+0] = 0xFF // B
		xor[i*4+1] = 0xAA // G
		xor[i*4+2] = 0x00 // R
		xor[i*4+3] = 0xFF // A
	}
	buf.Write(xor)

	// AND 掩码 (全 0)
	andMask := make([]byte, andSize)
	buf.Write(andMask)

	icoData := buf.Bytes()
	icon, err := NewIconFromICO(icoData)
	if err != nil {
		t.Fatalf("NewIconFromICO failed: %v", err)
	}
	defer icon.Destroy()

	if icon.GetSmallHandle() == 0 {
		t.Errorf("expected non-zero handle from ICO parser")
	}
}

// 回归：目录项 offset+size 在 uint32 下回绕后曾通过边界检查，
// 随后把越界指针传给 CreateIconFromResourceEx。
func TestNewIconFromICORejectsWrappedOffset(t *testing.T) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(buf, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(buf, binary.LittleEndian, uint16(1)) // count: 1

	entry := make([]byte, 16)
	entry[0], entry[1] = 16, 16
	binary.LittleEndian.PutUint16(entry[4:6], 1)     // planes
	binary.LittleEndian.PutUint16(entry[6:8], 32)    // bit count
	binary.LittleEndian.PutUint32(entry[8:12], 0x20) // size
	binary.LittleEndian.PutUint32(entry[12:16], 0xFFFFFFF0)
	buf.Write(entry)
	buf.Write(make([]byte, 32)) // 保证 len(data) 足够大，使回绕后的和能通过旧检查

	icon, err := NewIconFromICO(buf.Bytes())
	if err == nil {
		icon.Destroy()
		t.Fatal("expected error for directory entry with wrapped offset")
	}
}
