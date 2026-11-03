package photos

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/platform/apperr"
)

func noisy(w, h int) *image.RGBA {
	r := rand.New(rand.NewSource(1))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), 255
	}
	return img
}

func jpegBytes(t *testing.T, img image.Image) []byte {
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}))
	return b.Bytes()
}

// withEXIF inserts an APP1/Exif segment holding a secret marker right after the JPEG start marker.
func withEXIF(jpg []byte, secret string) []byte {
	payload := append([]byte("Exif\x00\x00"), []byte(secret)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

// TC-PH-001 / 006: output is a small JPEG without metadata.
func TestProcess_StripsMetadataAndLimitsSize_TC_PH_006(t *testing.T) {
	in := withEXIF(jpegBytes(t, noisy(2400, 1800)), "GPS-SECRET-LAT-12.97")
	require.Contains(t, string(in), "GPS-SECRET-LAT-12.97")

	out, err := Process(in)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), maxStoredBytes, "NFR-8: at most 500 KB")
	assert.NotContains(t, string(out), "GPS-SECRET-LAT")
	assert.NotContains(t, string(out), "Exif")
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.LessOrEqual(t, max(cfg.Width, cfg.Height), 1280)
}

// TC-PH-002: PNG is accepted and stored as JPEG; transparency does not turn black.
func TestProcess_PNGBecomesJPEG_TC_PH_002(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100)) // fully transparent
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	out, err := Process(b.Bytes())
	require.NoError(t, err)
	dec, err := jpeg.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	r, g, bl, _ := dec.At(100, 50).RGBA()
	assert.Greater(t, r>>8, uint32(240), "transparent pixels become white")
	assert.Greater(t, g>>8, uint32(240))
	assert.Greater(t, bl>>8, uint32(240))
}

// TC-PH-004: judged by content, not by file name or header.
func TestProcess_RejectsOtherContent_TC_PH_004(t *testing.T) {
	for name, data := range map[string][]byte{
		"text":  []byte("this is not an image, just a text file renamed to .jpg"),
		"gif":   []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"),
		"html":  []byte("<html><script>alert(1)</script></html>"),
		"empty": {},
	} {
		_, err := Process(data)
		assert.ErrorIs(t, err, apperr.ErrUnsupportedMedia, name)
	}
}

// TC-SEC-006: a PNG that claims 20 000 × 20 000 px is refused before it is decoded.
func TestProcess_RejectsDecompressionBomb_TC_SEC_006(t *testing.T) {
	chunk := func(kind string, data []byte) []byte {
		var b bytes.Buffer
		_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(kind)
		b.Write(data)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
		return b.Bytes()
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 20000)
	binary.BigEndian.PutUint32(ihdr[4:], 20000)
	ihdr[8], ihdr[9] = 8, 0 // 8-bit grayscale
	png := append([]byte("\x89PNG\r\n\x1a\n"), chunk("IHDR", ihdr)...)
	png = append(png, chunk("IDAT", nil)...)
	png = append(png, chunk("IEND", nil)...)

	_, err := Process(png)
	assert.ErrorIs(t, err, apperr.ErrFileTooLarge)
}

func TestProcess_SmallImageKeptSmall(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		img.Set(x, x, color.Black)
	}
	out, err := Process(jpegBytes(t, img))
	require.NoError(t, err)
	assert.Less(t, len(out), 20_000)
}

// Fuzz: arbitrary bytes must never panic, only return an error or a valid JPEG (spec §14, T-401).
func FuzzProcess(f *testing.F) {
	f.Add([]byte("not an image"))
	f.Add([]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"))
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := Process(data)
		if err == nil {
			_, derr := jpeg.Decode(bytes.NewReader(out))
			if derr != nil {
				t.Fatalf("Process returned undecodable output: %v", derr)
			}
		}
	})
}
