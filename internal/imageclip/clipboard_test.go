package imageclip

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestImageFromDataNamesAndMeasuresPNG(t *testing.T) {
	data := encodePNG(t, image.NewRGBA(image.Rect(0, 0, 3, 2)))
	image, err := imageFromData(data, time.Date(2026, 7, 18, 14, 30, 12, 345_000_000, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if image.Filename != "screenshot-20260718-143012-345.png" || image.MIMEType != "image/png" || image.Width != 3 || image.Height != 2 {
		t.Fatalf("image = %#v", image)
	}
}

func TestImageFromDataRejectsOversizedDimensions(t *testing.T) {
	data := encodePNG(t, image.NewRGBA(image.Rect(0, 0, MaxImageDimension+1, 1)))
	_, err := imageFromData(data, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "pixel limit") {
		t.Fatalf("error = %v", err)
	}
}

func TestImageFromDataRejectsOversizedBytes(t *testing.T) {
	_, err := imageFromData(make([]byte, MaxImageBytes+1), time.Time{})
	if err == nil || !strings.Contains(err.Error(), "MiB limit") {
		t.Fatalf("error = %v", err)
	}
}

func encodePNG(t *testing.T, source image.Image) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, source); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
