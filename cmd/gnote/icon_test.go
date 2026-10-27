//go:build windows

package main

import (
	"debug/pe"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"
)

func TestEnsureGnoteICOFile(t *testing.T) {
	target := "icon.ico"
	err := EnsureGnoteICOFile(target)
	if err != nil {
		t.Fatalf("EnsureGnoteICOFile failed: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat icon.ico failed: %v", err)
	}
	if info.Size() < 500 {
		t.Errorf("expected icon.ico size > 500 bytes, got %d", info.Size())
	}
}

func TestPEFileHasIconResource(t *testing.T) {
	exePath := "../../bin/gnote.exe"
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		exePath = "../../gnote.exe"
		if _, err := os.Stat(exePath); os.IsNotExist(err) {
			t.Skip("gnote.exe not yet built")
		}
	}

	f, err := pe.Open(exePath)
	if err != nil {
		t.Fatalf("failed to open PE file: %v", err)
	}
	defer f.Close()

	if opt, ok := f.OptionalHeader.(*pe.OptionalHeader64); ok {
		if opt.Subsystem != 2 { // 2 = IMAGE_SUBSYSTEM_WINDOWS_GUI
			t.Errorf("expected PE Subsystem to be 2 (IMAGE_SUBSYSTEM_WINDOWS_GUI, terminal hidden), got %d", opt.Subsystem)
		}
	}

	foundRsrc := false
	for _, sec := range f.Sections {
		if sec.Name == ".rsrc" {
			foundRsrc = true
			if sec.Size == 0 {
				t.Errorf(".rsrc section is empty")
			}
			break
		}
	}

	if !foundRsrc {
		t.Fatalf("expected .rsrc section in gnote.exe for Windows Explorer icon")
	}

	// 进一步通过 Windows Shell32 API 检验系统是否能成功提取图标
	absPath, _ := filepath.Abs(exePath)
	shell32 := syscall.NewLazyDLL("shell32.dll")
	extractIconW := shell32.NewProc("ExtractIconW")
	pathPtr := syscall.StringToUTF16Ptr(absPath)
	hIcon, _, _ := extractIconW.Call(0, uintptr(unsafe.Pointer(pathPtr)), 0)
	if hIcon <= 1 {
		t.Fatalf("Windows ExtractIconW failed to extract icon from gnote.exe, code=%d", hIcon)
	}
	defer syscall.NewLazyDLL("user32.dll").NewProc("DestroyIcon").Call(hIcon)
}
