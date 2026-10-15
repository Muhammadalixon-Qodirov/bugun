package speech

import (
	"encoding/binary"
	"math"
	"testing"
)

// oggPage builds a minimal Ogg page header with a granule position.
func oggPage(granule uint64, payload string) []byte {
	b := make([]byte, 27)
	copy(b, "OggS")
	binary.LittleEndian.PutUint64(b[6:14], granule)
	return append(b, payload...)
}

func TestOggSeconds(t *testing.T) {
	voice := append(oggPage(0, "OpusHead........"), oggPage(48000*7/2, "audio")...) // 3.5 s
	if s, ok := OggSeconds(voice); !ok || math.Abs(s-3.5) > 1e-9 {
		t.Errorf("opus: %v %v", s, ok)
	}
	vorbisHead := "\x01vorbis\x00\x00\x00\x00\x01"
	rate := make([]byte, 4)
	binary.LittleEndian.PutUint32(rate, 44100)
	vorbis := append(oggPage(0, vorbisHead+string(rate)), oggPage(44100*2, "x")...)
	if s, ok := OggSeconds(vorbis); !ok || s != 2 {
		t.Errorf("vorbis: %v %v", s, ok)
	}
	if _, ok := OggSeconds([]byte("not ogg")); ok {
		t.Error("not ogg")
	}
	if f, ok := Detect(voice); !ok || f.Name != "ogg" {
		t.Errorf("detect %v", f)
	}
	for _, c := range []struct {
		b    []byte
		name string
	}{
		{[]byte{0x1A, 0x45, 0xDF, 0xA3, 1}, "webm"},
		{[]byte("\x00\x00\x00\x20ftypM4A "), "mp4"},
		{[]byte("RIFF\x00\x00\x00\x00WAVEfmt "), "wav"},
		{[]byte("ID3\x04"), "mp3"},
	} {
		if f, ok := Detect(c.b); !ok || f.Name != c.name {
			t.Errorf("%s: %v", c.name, f)
		}
	}
	if _, ok := Detect([]byte("%PDF-1.4")); ok {
		t.Error("a PDF is not audio")
	}
	if MaxBytes(60) < 1_000_000 {
		t.Error("60 s at 128 kbit/s must fit")
	}
}
