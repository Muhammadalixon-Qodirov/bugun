// Package speech checks voice recordings before they go to a speech-to-text
// model (spec AI-1): the format from the file's first bytes, the length of an
// OGG recording (Telegram voice messages, Firefox) from its last page, and a
// size bound derived from the allowed length.
package speech

import (
	"bytes"
	"encoding/binary"
)

// Format is a recognised audio container.
type Format struct {
	Name string // ogg, webm, mp4, wav, mp3
	Ext  string // file name extension for the model's API
}

// Detect recognises the audio formats browsers and Telegram record.
func Detect(b []byte) (Format, bool) {
	switch {
	case bytes.HasPrefix(b, []byte("OggS")):
		return Format{"ogg", "ogg"}, true
	case bytes.HasPrefix(b, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return Format{"webm", "webm"}, true
	case len(b) >= 12 && bytes.Equal(b[4:8], []byte("ftyp")):
		return Format{"mp4", "m4a"}, true
	case len(b) >= 12 && bytes.HasPrefix(b, []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WAVE")):
		return Format{"wav", "wav"}, true
	case bytes.HasPrefix(b, []byte("ID3")), len(b) >= 2 && b[0] == 0xFF && b[1]&0xE0 == 0xE0:
		return Format{"mp3", "mp3"}, true
	}
	return Format{}, false
}

// OggSeconds is the length of an Ogg Opus or Vorbis recording: the granule
// position of the last page over the sample rate. ok is false when it
// cannot tell.
func OggSeconds(b []byte) (float64, bool) {
	if !bytes.HasPrefix(b, []byte("OggS")) {
		return 0, false
	}
	rate := 0.0
	switch {
	case bytes.Contains(b[:min(len(b), 512)], []byte("OpusHead")):
		rate = 48000 // Opus granules always count 48 kHz samples
	default:
		if i := bytes.Index(b[:min(len(b), 512)], []byte("\x01vorbis")); i >= 0 && len(b) >= i+16 {
			rate = float64(binary.LittleEndian.Uint32(b[i+12 : i+16]))
		}
	}
	if rate <= 0 {
		return 0, false
	}
	last := bytes.LastIndex(b, []byte("OggS"))
	if last < 0 || len(b) < last+14 {
		return 0, false
	}
	granule := binary.LittleEndian.Uint64(b[last+6 : last+14])
	if granule == ^uint64(0) {
		return 0, false
	}
	return float64(granule) / rate, true
}

// MaxBytes bounds a recording of at most seconds: 256 kbit/s, above what
// voice recorders use, plus room for headers.
func MaxBytes(seconds int) int64 { return int64(seconds)*32_000 + 64<<10 }
