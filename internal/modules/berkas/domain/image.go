package domain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// Content-type boundaries for accepted uploads.
const (
	mimeJPEG = "image/jpeg"
	mimePNG  = "image/png"
	mimeWebP = "image/webp"
)

var (
	pngSignature = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	// Succesful errors surfaced when bytes are structurally invalid.
	errInvalidJPEG = errors.New("invalid jpeg bytes")
	errInvalidPNG  = errors.New("invalid png bytes")
	errInvalidWebP = errors.New("invalid webp bytes")
)

// DetectImageType identifies the image format from magic bytes only
// (never the file extension). Returns the mime type (used for the
// berkas row), the extension used in the object key, and ok=false when
// the bytes do not match JPEG, PNG, or WebP signatures.
func DetectImageType(b []byte) (mime string, ext string, ok bool) {
	// JPEG: FF D8 FF.
	if len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return mimeJPEG, "jpg", true
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A.
	if bytes.HasPrefix(b, pngSignature) {
		return mimePNG, "png", true
	}
	// WebP: "RIFF" + 4-byte size + "WEBP".
	if len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return mimeWebP, "webp", true
	}
	return "", "", false
}

// StripImageMetadata removes EXIF (and WebP XMP) metadata server-side
// at the byte level — no re-encoding, so pixels stay untouched:
//   - JPEG: drop every APP1 (Exif/XMP) segment;
//   - PNG: drop every eXIf chunk;
//   - WebP: drop EXIF and "XMP " chunks and rewrite the RIFF size.
//
// Structurally invalid bytes return an error rather than being stored.
func StripImageMetadata(b []byte, mime string) ([]byte, error) {
	switch mime {
	case mimeJPEG:
		return stripJPEGMetadata(b)
	case mimePNG:
		return stripPNGMetadata(b)
	case mimeWebP:
		return stripWebPMetadata(b)
	default:
		// Only called after DetectImageType, so this is a programming error.
		return nil, fmt.Errorf("strip metadata: unsupported mime %q", mime)
	}
}

// stripJPEGMetadata copies every marker segment except APP1 (FFE1).
func stripJPEGMetadata(b []byte) ([]byte, error) {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return nil, errInvalidJPEG
	}
	out := make([]byte, 0, len(b))
	out = append(out, b[0:2]...) // SOI
	i := 2
	for i < len(b) {
		start := i // segment start (fill bytes included)
		// Skip fill bytes between segments (0xFF padding).
		for i < len(b) && b[i] == 0xFF {
			i++
		}
		if i >= len(b) {
			return nil, errInvalidJPEG
		}
		marker := b[i]
		i++
		switch {
		case marker == 0xD9: // EOI
			out = append(out, 0xFF, 0xD9)
			if i < len(b) {
				out = append(out, b[i:]...) // preserve any trailing bytes
			}
			return out, nil
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			// TEM / RSTn: no length payload.
			out = append(out, 0xFF, marker)
		case marker == 0xDA: // SOS: entropy data follows verbatim.
			if i+2 > len(b) {
				return nil, errInvalidJPEG
			}
			out = append(out, 0xFF, 0xDA)
			out = append(out, b[i:]...)
			return out, nil
		default:
			if i+2 > len(b) {
				return nil, errInvalidJPEG
			}
			segLen := int(b[i])<<8 | int(b[i+1]) // includes its own 2 bytes
			if segLen < 2 || i+segLen > len(b) {
				return nil, errInvalidJPEG
			}
			if marker != 0xE1 { // drop APP1 (Exif/XMP), copy the rest
				out = append(out, b[start:i+segLen]...)
			}
			i += segLen
		}
	}
	return nil, errInvalidJPEG // no EOI found
}

// stripPNGMetadata copies every chunk except eXIf.
func stripPNGMetadata(b []byte) ([]byte, error) {
	if len(b) < 8+8+4 { // sig + chunk header + CRC at minimum
		return nil, errInvalidPNG
	}
	if !bytes.HasPrefix(b, pngSignature) {
		return nil, errInvalidPNG
	}
	out := make([]byte, 0, len(b))
	out = append(out, b[:8]...)
	i := 8
	for i+8 <= len(b) {
		chunkLen := int(binary.BigEndian.Uint32(b[i : i+4]))
		typ := string(b[i+4 : i+8])
		end := i + 8 + chunkLen + 4 // data + CRC
		if chunkLen < 0 || end < i || end > len(b) {
			return nil, errInvalidPNG
		}
		if typ != "eXIf" { // keep header/data chunks with their CRC
			out = append(out, b[i:end]...)
		}
		if typ == "IEND" { // trailing garbage is preserved verbatim
			out = append(out, b[end:]...)
			return out, nil
		}
		i = end
	}
	return nil, errInvalidPNG // IEND not reached
}

// stripWebPMetadata copies every chunk except EXIF / XMP and rewrites
// the RIFF size to the new total.
func stripWebPMetadata(b []byte) ([]byte, error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return nil, errInvalidWebP
	}
	riffSize := int(binary.LittleEndian.Uint32(b[4:8]))
	if riffSize < 4 || 8+riffSize != len(b) {
		return nil, errInvalidWebP
	}
	out := make([]byte, 12, len(b))
	copy(out, "RIFF")
	copy(out[8:12], "WEBP")
	i := 12
	for i < len(b) {
		if i+8 > len(b) {
			return nil, errInvalidWebP
		}
		fourcc := string(b[i : i+4])
		chunkSize := int(binary.LittleEndian.Uint32(b[i+4 : i+8]))
		end := i + 8 + chunkSize
		if chunkSize < 0 || end < i || end > len(b) {
			return nil, errInvalidWebP
		}
		if fourcc != "EXIF" && fourcc != "XMP " {
			payload := b[i:end]
			out = append(out, payload...)
			if chunkSize%2 == 1 { // chunks are padded to even sizes
				out = append(out, 0)
			}
		}
		i = end
		if chunkSize%2 == 1 && i < len(b) { // pad byte
			i++
		}
	}
	if i != len(b) {
		return nil, errInvalidWebP
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8)) // rewrite RIFF size
	return out, nil
}
