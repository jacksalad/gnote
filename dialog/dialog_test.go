//go:build windows

package dialog

import (
	"syscall"
	"testing"
)

func TestBuildFilterDoubleNullTerminated(t *testing.T) {
	buf := buildFilter("文本文件 (*.txt)|*.txt|所有文件 (*.*)|*.*")
	if len(buf) < 4 || buf[len(buf)-1] != 0 || buf[len(buf)-2] != 0 {
		t.Fatalf("filter must end with double NUL, got %v", buf)
	}
	if got, want := syscall.UTF16ToString(buf), "文本文件 (*.txt)"; got != want {
		t.Fatalf("first filter item = %q, want %q", got, want)
	}

	empty := buildFilter("")
	if len(empty) < 2 || empty[len(empty)-1] != 0 || empty[len(empty)-2] != 0 {
		t.Fatalf("default filter must be double-NUL terminated: %v", empty)
	}
}

func TestCopyUTF16Name(t *testing.T) {
	buf := make([]uint16, 32)
	copyUTF16Name(buf, "未命名.txt")
	if got, want := syscall.UTF16ToString(buf), "未命名.txt"; got != want {
		t.Fatalf("copied name = %q, want %q", got, want)
	}

	// 缓冲区只够放一个 emoji（2 个码元）：必须 NUL 结尾且不留下孤立代理
	buf = make([]uint16, 4)
	copyUTF16Name(buf, "😀😀")
	if buf[0] != 0xD83D || buf[1] != 0xDE00 || buf[2] != 0 {
		t.Fatalf("truncated name must not split a surrogate pair: %v", buf[:3])
	}

	buf = make([]uint16, 8)
	copyUTF16Name(buf, "")
	for i, u := range buf {
		if u != 0 {
			t.Fatalf("empty name must leave buffer untouched at %d: %v", i, buf)
		}
	}
}
