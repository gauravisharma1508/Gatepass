package photos

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder
	"net/http"

	"github.com/disintegration/imaging"

	"gatepass/internal/platform/apperr"
)

const (
	MaxUploadBytes = 5 << 20 // 5 MB
	maxSidePx      = 8000    // larger images are rejected before decoding (decompression-bomb guard)
	fitPx          = 1280
	maxStoredBytes = 500_000 // NFR-8: ≤ 500 KB
)

// Process validates by content (not by header), fixes orientation, shrinks and re-encodes as JPEG.
// Re-encoding from pixels drops all metadata (EXIF/GPS).
func Process(data []byte) ([]byte, error) {
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png":
	default:
		return nil, apperr.ErrUnsupportedMedia
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.ErrUnsupportedMedia
	}
	if cfg.Width > maxSidePx || cfg.Height > maxSidePx {
		return nil, apperr.ErrFileTooLarge.With("Image is larger than 8000 × 8000 pixels")
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, apperr.ErrUnsupportedMedia
	}
	b := img.Bounds()
	if b.Dx() > fitPx || b.Dy() > fitPx {
		img = imaging.Fit(img, fitPx, fitPx, imaging.Lanczos)
	}
	// flatten transparency (PNG) onto white so it does not turn black in JPEG
	bg := imaging.New(img.Bounds().Dx(), img.Bounds().Dy(), color.White)
	img = imaging.OverlayCenter(bg, img, 1.0)

	for size := fitPx; size >= 320; size = size * 3 / 4 {
		for q := 80; q >= 40; q -= 10 {
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
				return nil, err
			}
			if buf.Len() <= maxStoredBytes {
				return buf.Bytes(), nil
			}
		}
		img = imaging.Fit(img, size*3/4, size*3/4, imaging.Lanczos)
	}
	return nil, apperr.ErrFileTooLarge.With("Image cannot be compressed enough")
}
