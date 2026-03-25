package ssh

import (
	"encoding/binary"
	"testing"
)

func TestGenerateSigner(t *testing.T) {
	signer, err := generateSigner()
	if err != nil {
		t.Fatal(err)
	}
	if signer == nil {
		t.Fatal("expected non-nil signer")
	}
	if got := signer.PublicKey().Type(); got != "ssh-ed25519" {
		t.Fatalf("expected ssh-ed25519 key type, got %s", got)
	}
}

func TestParsePtyRequest(t *testing.T) {
	// Build a valid pty-req payload: string(term) + uint32(width) + uint32(height)
	term := "xterm-256color"
	width := uint32(120)
	height := uint32(40)

	buf := make([]byte, 4+len(term)+4+4)
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(term)))
	copy(buf[4:4+len(term)], term)
	off := 4 + len(term)
	binary.BigEndian.PutUint32(buf[off:off+4], width)
	binary.BigEndian.PutUint32(buf[off+4:off+8], height)

	pty, ok := parsePtyRequest(buf)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if pty.Term != term {
		t.Fatalf("term = %q; want %q", pty.Term, term)
	}
	if pty.Window.Width != int(width) {
		t.Fatalf("width = %d; want %d", pty.Window.Width, width)
	}
	if pty.Window.Height != int(height) {
		t.Fatalf("height = %d; want %d", pty.Window.Height, height)
	}
}

func TestParsePtyRequestTooShort(t *testing.T) {
	_, ok := parsePtyRequest([]byte{0, 0})
	if ok {
		t.Fatal("expected ok=false for short input")
	}
}

func TestParseWinchRequest(t *testing.T) {
	width := uint32(200)
	height := uint32(50)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], width)
	binary.BigEndian.PutUint32(buf[4:8], height)

	win, ok := parseWinchRequest(buf)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if win.Width != int(width) {
		t.Fatalf("width = %d; want %d", win.Width, width)
	}
	if win.Height != int(height) {
		t.Fatalf("height = %d; want %d", win.Height, height)
	}
}

func TestParseWinchRequestZeroDimension(t *testing.T) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], 0) // zero width
	binary.BigEndian.PutUint32(buf[4:8], 50)

	_, ok := parseWinchRequest(buf)
	if ok {
		t.Fatal("expected ok=false for zero width")
	}
}

func TestParseWinchRequestTooShort(t *testing.T) {
	_, ok := parseWinchRequest([]byte{0, 0})
	if ok {
		t.Fatal("expected ok=false for short input")
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantStr string
		wantOk  bool
	}{
		{"valid", append([]byte{0, 0, 0, 5}, "hello"...), "hello", true},
		{"empty string", []byte{0, 0, 0, 0}, "", true},
		{"too short header", []byte{0, 0}, "", false},
		{"length exceeds data", append([]byte{0, 0, 0, 10}, "hi"...), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, ok := parseString(tt.input)
			if ok != tt.wantOk {
				t.Fatalf("ok = %v; want %v", ok, tt.wantOk)
			}
			if ok && out != tt.wantStr {
				t.Fatalf("out = %q; want %q", out, tt.wantStr)
			}
		})
	}
}

func TestParseUint32(t *testing.T) {
	buf := []byte{0, 0, 1, 0, 0xFF}
	v, rest, ok := parseUint32(buf)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if v != 256 {
		t.Fatalf("v = %d; want 256", v)
	}
	if len(rest) != 1 {
		t.Fatalf("rest len = %d; want 1", len(rest))
	}

	_, _, ok = parseUint32([]byte{0, 0})
	if ok {
		t.Fatal("expected ok=false for short input")
	}
}
