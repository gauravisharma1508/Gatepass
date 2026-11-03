package testutil

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// JPEG returns a noisy JPEG of the given size (big files, hard to compress).
func JPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	r := rand.New(rand.NewSource(7))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), 255
	}
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, img, &jpeg.Options{Quality: 90}))
	return b.Bytes()
}

// WithEXIF inserts an APP1/Exif segment containing secret right after the JPEG start marker.
func WithEXIF(jpg []byte, secret string) []byte {
	payload := append([]byte("Exif\x00\x00"), []byte(secret)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

// PlannedBody is a valid planned-visit request for today; override fields as needed.
func PlannedBody(over map[string]any) map[string]any {
	b := map[string]any{"visitorName": "Ravi Kumar", "visitorMobile": "9876543210", "visitorCompany": "Acme",
		"purpose": "Vendor discussion", "visitDate": "2026-11-03", "expectedDurationMin": 60}
	for k, v := range over {
		b[k] = v
	}
	return b
}

// WalkInBody is a valid walk-in request for the given host id.
func WalkInBody(hostID string, over map[string]any) map[string]any {
	b := map[string]any{"visitorName": "Ravi Kumar", "visitorMobile": "9876543210", "visitorCompany": "Acme",
		"purpose": "Vendor discussion", "expectedDurationMin": 60, "hostId": hostID}
	for k, v := range over {
		b[k] = v
	}
	return b
}
