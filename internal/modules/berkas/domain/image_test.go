package domain

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// --- deterministic byte builders for tests -------------------------------

// buildJPEG returns a minimal structurally valid JPEG:
// SOI, APP0 (kept), APP1/Exif (dropped), a Comment (kept), SOS with
// entropy data, EOI.
func buildJPEG(exifPayload string) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8}) // SOI
	seg := func(marker byte, payload []byte) {
		b.WriteByte(0xFF)
		b.WriteByte(marker)
		l := len(payload) + 2
		b.WriteByte(byte(l >> 8))
		b.WriteByte(byte(l))
		b.Write(payload)
	}
	seg(0xE0, []byte("JFIF\x00\x01")) // APP0 retained
	if exifPayload != "" {
		seg(0xE1, []byte("Exif\x00\x00"+exifPayload)) // APP1/Exif
	}
	seg(0xFE, []byte("comment")) // COM retained
	// SOS with a small entropy payload; must be copied verbatim.
	b.Write([]byte{0xFF, 0xDA, 0x00, 0x06, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
	b.Write([]byte{0x12, 0xFF, 0x00, 0x55}) // fake entropy incl. stuffed FF00
	b.Write([]byte{0xFF, 0xD9})             // EOI
	return b.Bytes()
}

// buildPNG returns a minimal PNG: sig + IHDR + (optional) eXIf + IDAT + IEND.
func buildPNG(withExif bool) []byte {
	chunks := func(b *bytes.Buffer, typ string, data []byte) {
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
		b.Write(lenBuf[:])
		b.WriteString(typ)
		b.Write(data)
		b.Write([]byte{0, 0, 0, 0}) // dummy CRC
	}
	var b bytes.Buffer
	b.Write(pngSignature)
	chunks(&b, "IHDR", []byte{1, 0, 0, 1, 8, 6, 0, 0, 0, 0, 13, 0}) // 13-byte IHDR payload
	if withExif {
		chunks(&b, "eXIf", []byte("exif-data-goes-here"))
	}
	chunks(&b, "IDAT", []byte("pixel-data"))
	chunks(&b, "IEND", nil)
	return b.Bytes()
}

// buildWebP returns a minimal RIFF container: RIFF header + WEBP + listed
// fourcc/data pairs, computing the RIFF size automatically.
func buildWebP(t *testing.T, chunks ...[2]interface{}) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range chunks {
		typ := c[0].(string)
		data := c[1].([]byte)
		body.WriteString(typ)
		var sizeBuf [4]byte
		total := uint32(len(data))
		if len(data)%2 == 1 {
			total++ // pad byte
		}
		binary.LittleEndian.PutUint32(sizeBuf[:], total)
		body.Write(sizeBuf[:])
		body.Write(data)
		if len(data)%2 == 1 {
			body.WriteByte(0)
		}
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	var riff [4]byte
	binary.LittleEndian.PutUint32(riff[:], uint32(body.Len()))
	out.Write(riff[:])
	out.Write(body.Bytes())
	return out.Bytes()
}

// walkJPEGSegments parses the copied output and returns the markers seen
// before SOS; fails the test when the byte stream is not parseable.
func walkJPEGSegments(t *testing.T, b []byte) []byte {
	t.Helper()
	if b[0] != 0xFF || b[1] != 0xD8 {
		t.Fatalf("missing SOI")
	}
	i := 2
	var markers []byte
	for i < len(b) {
		if b[i] != 0xFF {
			t.Fatalf("expected marker byte 0xFF at %d, got %#x", i, b[i])
		}
		for i < len(b) && b[i] == 0xFF {
			i++
		}
		if i >= len(b) {
			t.Fatalf("truncated marker stream")
		}
		m := b[i]
		i++
		markers = append(markers, m)
		switch {
		case m == 0xD9:
			if i != len(b) {
				t.Fatalf("bytes after EOI: %d", len(b)-i)
			}
			return markers
		case m == 0xDA:
			return markers
		case m == 0x01 || (m >= 0xD0 && m <= 0xD7):
			// no payload
		default:
			if i+2 > len(b) {
				t.Fatalf("truncated length for marker %#x", m)
			}
			segLen := int(b[i])<<8 | int(b[i+1])
			if segLen < 2 || i+segLen > len(b) {
				t.Fatalf("bad length %d for marker %#x", segLen, m)
			}
			i += segLen
		}
	}
	t.Fatalf("EOI not found")
	return markers
}

// --- DetectImageType ------------------------------------------------------

func TestDetectImageType(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		mime string
		ext  string
		ok   bool
	}{
		{"jpeg", append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, 0x00, 0x10, 'J', 'F', 'I', 'F'), "image/jpeg", "jpg", true},
		{"png", pngSignature, "image/png", "png", true},
		{"webp riff missing", []byte("RIFX\x00\x00\x00\x00WEBPVP8 "), "", "", false},
		{"webp", buildWebP(t, [2]interface{}{"VP8 ", []byte("data12345")}), "image/webp", "webp", true},
		{"gif", []byte("GIF89a\x01\x00\x01\x00"), "", "", false},
		{"text", []byte("hello world"), "", "", false},
		{"empty", nil, "", "", false},
		{"truncated webp", []byte("RIFF\x00\x00\x00"), "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mime, ext, ok := DetectImageType(tc.in)
			if ok != tc.ok || mime != tc.mime || ext != tc.ext {
				t.Fatalf("DetectImageType = (%q, %q, %v), want (%q, %q, %v)", mime, ext, ok, tc.mime, tc.ext, tc.ok)
			}
		})
	}
}

// --- StripImageMetadata: JPEG ---------------------------------------------

func TestStripImageMetadataJPEGDropsAPP1(t *testing.T) {
	in := buildJPEG("EXIF GPS and camera time")
	if bytes.Contains(in, []byte{0xE1}) != true {
		t.Fatalf("test fixture must contain APP1")
	}
	out, err := StripImageMetadata(in, "image/jpeg")
	if err != nil {
		t.Fatalf("strip jpeg: %v", err)
	}
	// APP1 gone, APP0 + COM kept, structure parseable.
	walkJPEGSegments(t, out)
	if bytes.Contains(out, []byte("Exif\x00\x00")) {
		t.Fatalf("EXIF payload still present")
	}
	for _, want := range []byte("JFIF") {
		if !bytes.Contains(out, []byte{want}) {
			t.Fatalf("APP0 JFIF lost during strip")
		}
	}
	if !bytes.Contains(out, []byte("comment")) {
		t.Fatalf("COM segment lost during strip")
	}
	if !bytes.Contains(out, []byte{0xFF, 0xDA, 0x00, 0x06, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}) {
		t.Fatalf("SOS segment lost during strip")
	}
}

func TestStripImageMetadataJPEGWithoutEXIFIsMostlyUnchanged(t *testing.T) {
	in := buildJPEG("")
	out, err := StripImageMetadata(in, "image/jpeg")
	if err != nil {
		t.Fatalf("strip jpeg: %v", err)
	}
	if !bytes.Equal(in, out) {
		t.Fatalf("expected identical bytes when no APP1 present")
	}
}

func TestStripImageMetadataJPEGInvalid(t *testing.T) {
	cases := map[string][]byte{
		"truncated":    {0xFF, 0xD8, 0xFF},
		"no soi":       {0x00, 0x01, 0x02, 0x03},
		"no eoi":       {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x04, 'x', 'y'},
		"bad length":   {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x01, 0xFF, 0xD9},
		"empty":        {},
		"length lower": {0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x01},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := StripImageMetadata(append([]byte{}, in...), "image/jpeg"); err == nil {
				t.Fatalf("expected error for invalid jpeg")
			}
		})
	}
}

// --- StripImageMetadata: PNG ----------------------------------------------

func TestStripImageMetadataPNGDropsEXIFChunk(t *testing.T) {
	in := buildPNG(true)
	// Panjang chunk eXIf ada di input tapi hilang di output chunk scan.
	countChunks := func(b []byte) []string {
		var types []string
		for i := 8; i+8 <= len(b); {
			l := int(binary.BigEndian.Uint32(b[i : i+4]))
			typ := string(b[i+4 : i+8])
			types = append(types, typ)
			i += 8 + l + 4
		}
		return types
	}
	out, err := StripImageMetadata(in, "image/png")
	if err != nil {
		t.Fatalf("strip png: %v", err)
	}
	got := strings.Join(countChunks(out), ",")
	want := "IHDR,IDAT,IEND"
	if got != want {
		t.Fatalf("chunks after strip = %q, want %q", got, want)
	}
	if bytes.Contains(out, []byte("eXIf")) {
		t.Fatalf("eXIf chunk still present")
	}
	if !bytes.Contains(out, []byte("pixel-data")) {
		t.Fatalf("IDAT lost during strip")
	}
}

func TestStripImageMetadataPNGNoEXIF(t *testing.T) {
	in := buildPNG(false)
	out, err := StripImageMetadata(in, "image/png")
	if err != nil {
		t.Fatalf("strip png: %v", err)
	}
	if !bytes.Equal(in, out) {
		t.Fatalf("output should equal input when there is no eXIf chunk")
	}
}

// --- StripImageMetadata: WebP ---------------------------------------------

func TestStripImageMetadataWebPDropsEXIFAndXMPAndRewritesRIFFSize(t *testing.T) {
	in := buildWebP(t,
		[2]interface{}{"VP8X", []byte("1234567890")},
		[2]interface{}{"EXIF", []byte("metadata-to-drop")},
		[2]interface{}{"XMP ", []byte("xmp-to-drop")},
		[2]interface{}{"VP8 ", []byte("pixel-bytes-here")},
	)
	if bytes.Count(in, []byte("EXIF")) != 1 {
		t.Fatalf("fixture must contain EXIF chunk")
	}
	out, err := StripImageMetadata(in, "image/webp")
	if err != nil {
		t.Fatalf("strip webp: %v", err)
	}
	if string(out[0:4]) != "RIFF" || string(out[8:12]) != "WEBP" {
		t.Fatalf("RIFF/WEBP header lost: %q", out[:12])
	}
	riffSize := int(binary.LittleEndian.Uint32(out[4:8]))
	if riffSize != len(out)-8 {
		t.Fatalf("RIFF size %d != len(out)-8 (%d)", riffSize, len(out)-8)
	}
	if bytes.Contains(out, []byte("metadata-to-drop")) || bytes.Contains(out, []byte("xmp-to-drop")) {
		t.Fatalf("EXIF/XMP payload still present")
	}
	if !bytes.Contains(out, []byte("pixel-bytes-here")) {
		t.Fatalf("VP8 chunk lost during strip")
	}
	// Parse chunks of the output to verify the structure walked cleanly.
	i := 12
	for i < len(out) {
		typ := string(out[i : i+4])
		size := int(binary.LittleEndian.Uint32(out[i+4 : i+8]))
		i += 8 + size
		if size%2 == 1 {
			i++ // pad byte
		}
		switch typ {
		case "VP8X", "VP8 ":
		default:
			t.Fatalf("unexpected chunk %q remains", typ)
		}
	}
	if i != len(out) {
		t.Fatalf("chunk walk overflow")
	}
}

func TestStripImageMetadataWebPInvalid(t *testing.T) {
	cases := map[string][]byte{
		"not riff":      []byte("WAVEBWEBPVP8 "),
		"truncated hdr": []byte("RIFF"),
		"bad riff size": func() []byte {
			b := buildWebP(t, [2]interface{}{"VP8 ", []byte("12345")})
			binary.LittleEndian.PutUint32(b[4:8], 999)
			return b
		}(),
		"overrun chunk": func() []byte {
			b := buildWebP(t, [2]interface{}{"VP8 ", []byte("12345")})
			binary.LittleEndian.PutUint32(b[16:20], 999)
			return b
		}(),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := StripImageMetadata(append([]byte{}, in...), "image/webp"); err == nil {
				t.Fatalf("expected error for invalid webp")
			}
		})
	}
}

// StripImageMetadata must reject formats that were never detected.
func TestStripImageMetadataUnknownMime(t *testing.T) {
	if _, err := StripImageMetadata([]byte("GIF89a"), "image/gif"); err == nil {
		t.Fatalf("expected error for unsupported mime")
	}
}
