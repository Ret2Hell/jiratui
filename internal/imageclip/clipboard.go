// Package imageclip reads image data from the operating system clipboard.
package imageclip

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	// MaxImageBytes is the largest clipboard image jiratui will read.
	MaxImageBytes = 25 << 20
	// MaxImageDimension is the largest supported width or height in pixels.
	MaxImageDimension = 16_384
	maxTypeListBytes  = 64 << 10
	maxBase64Bytes    = (MaxImageBytes*4)/3 + 8
)

// Image is an image read from the clipboard.
type Image struct {
	Filename string
	MIMEType string
	Data     []byte
	Width    int
	Height   int
}

// Reader reads an image from the system clipboard.
type Reader interface {
	ReadImage(context.Context) (Image, error)
}

// SystemReader uses the clipboard tools available on the current platform.
type SystemReader struct{}

// ReadImage reads PNG, JPEG, or GIF data from the system clipboard.
func (SystemReader) ReadImage(ctx context.Context) (Image, error) {
	data, err := readClipboard(ctx)
	if err != nil {
		return Image{}, err
	}
	return imageFromData(data, time.Now())
}

func imageFromData(data []byte, now time.Time) (Image, error) {
	if len(data) > MaxImageBytes {
		return Image{}, fmt.Errorf("clipboard image exceeds the %d MiB limit", MaxImageBytes>>20)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Image{}, fmt.Errorf("clipboard does not contain a supported image: %w", err)
	}
	ext, mimeType := imageFormat(format)
	if ext == "" {
		return Image{}, fmt.Errorf("clipboard image format %q is not supported", format)
	}
	if config.Width > MaxImageDimension || config.Height > MaxImageDimension {
		return Image{}, fmt.Errorf("clipboard image dimensions %dx%d exceed the %d-pixel limit", config.Width, config.Height, MaxImageDimension)
	}
	timestamp := strings.ReplaceAll(now.Format("20060102-150405.000"), ".", "-")
	return Image{
		Filename: fmt.Sprintf("screenshot-%s.%s", timestamp, ext),
		MIMEType: mimeType,
		Data:     data,
		Width:    config.Width,
		Height:   config.Height,
	}, nil
}

func readClipboard(ctx context.Context) ([]byte, error) {
	switch runtime.GOOS {
	case "linux":
		return readLinux(ctx)
	case "darwin":
		if _, err := exec.LookPath("pngpaste"); err != nil {
			return nil, errors.New("image paste on macOS requires pngpaste (brew install pngpaste)")
		}
		return commandOutput(ctx, MaxImageBytes, "pngpaste", "-")
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms; $i=[Windows.Forms.Clipboard]::GetImage(); if($null -eq $i){exit 2}; $m=New-Object IO.MemoryStream; $i.Save($m,[Drawing.Imaging.ImageFormat]::Png); [Console]::Write([Convert]::ToBase64String($m.ToArray()))`
		encoded, err := commandOutput(ctx, maxBase64Bytes, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
		if err != nil {
			return nil, errors.New("clipboard does not contain an image")
		}
		data, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil {
			return nil, fmt.Errorf("decode Windows clipboard image: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("image clipboard is not supported on %s", runtime.GOOS)
	}
}

func readLinux(ctx context.Context) ([]byte, error) {
	if _, err := exec.LookPath("wl-paste"); err == nil && strings.TrimSpace(string(commandOutputIgnoringError(ctx, maxTypeListBytes, "wl-paste", "--list-types"))) != "" {
		for _, mimeType := range []string{"image/png", "image/jpeg", "image/gif"} {
			if data, err := commandOutput(ctx, MaxImageBytes, "wl-paste", "--no-newline", "--type", mimeType); err == nil && len(data) > 0 {
				return data, nil
			}
		}
		return nil, errors.New("clipboard does not contain a supported image")
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		for _, mimeType := range []string{"image/png", "image/jpeg", "image/gif"} {
			if data, err := commandOutput(ctx, MaxImageBytes, "xclip", "-selection", "clipboard", "-t", mimeType, "-o"); err == nil && len(data) > 0 {
				return data, nil
			}
		}
		return nil, errors.New("clipboard does not contain a supported image")
	}
	return nil, errors.New("image paste requires wl-paste (Wayland) or xclip (X11)")
}

func commandOutput(ctx context.Context, limit int, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, int64(limit)+1))
	if len(data) > limit {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("clipboard image exceeds the %d MiB limit", MaxImageBytes>>20)
	}
	waitErr := command.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil {
		return nil, waitErr
	}
	if len(data) == 0 {
		return nil, errors.New("clipboard is empty")
	}
	return data, nil
}

func commandOutputIgnoringError(ctx context.Context, limit int, name string, args ...string) []byte {
	data, _ := commandOutput(ctx, limit, name, args...)
	return data
}

func imageFormat(format string) (extension, mimeType string) {
	switch format {
	case "png":
		return "png", "image/png"
	case "jpeg":
		return "jpg", "image/jpeg"
	case "gif":
		return "gif", "image/gif"
	default:
		return "", ""
	}
}
