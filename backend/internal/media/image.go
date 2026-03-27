package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

var (
	ErrFileTooLarge      = errors.New("image file is too large")
	ErrUnsupportedFormat = errors.New("unsupported image format")
)

type OptimizeOptions struct {
	MaxBytes    int64
	MaxWidth    int
	MaxHeight   int
	JPEGQuality int
}

type OptimizedImage struct {
	Extension string
	Width     int
	Height    int
}

func OptimizeAndSaveUpload(header *multipart.FileHeader, destinationPrefix string, options OptimizeOptions) (OptimizedImage, error) {
	sourceBytes, contentType, err := readUpload(header, options.MaxBytes)
	if err != nil {
		return OptimizedImage{}, err
	}

	img, err := decodeImage(sourceBytes, contentType)
	if err != nil {
		return OptimizedImage{}, err
	}

	optimized := resizeToFit(img, options.MaxWidth, options.MaxHeight)

	extension := ".jpg"
	if shouldEncodeAsPNG(contentType, optimized) {
		extension = ".png"
	}

	if err := os.MkdirAll(filepath.Dir(destinationPrefix), 0o755); err != nil {
		return OptimizedImage{}, fmt.Errorf("create uploads directory: %w", err)
	}

	destinationPath := destinationPrefix + extension
	if err := encodeImage(destinationPath, optimized, extension, options.JPEGQuality); err != nil {
		return OptimizedImage{}, err
	}

	bounds := optimized.Bounds()
	return OptimizedImage{
		Extension: extension,
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
	}, nil
}

func readUpload(header *multipart.FileHeader, maxBytes int64) ([]byte, string, error) {
	if header == nil {
		return nil, "", ErrUnsupportedFormat
	}

	if maxBytes > 0 && header.Size > maxBytes {
		return nil, "", ErrFileTooLarge
	}

	file, err := header.Open()
	if err != nil {
		return nil, "", fmt.Errorf("open uploaded image: %w", err)
	}
	defer file.Close()

	sourceBytes, err := readAtMost(file, maxBytes)
	if err != nil {
		return nil, "", err
	}

	contentType := http.DetectContentType(sourceBytes[:min(len(sourceBytes), 512)])
	return sourceBytes, contentType, nil
}

func readAtMost(file multipart.File, maxBytes int64) ([]byte, error) {
	var reader bytes.Buffer
	if maxBytes > 0 {
		limit := maxBytes + 1
		if _, err := reader.ReadFrom(io.LimitReader(file, limit)); err != nil {
			return nil, fmt.Errorf("read uploaded image: %w", err)
		}
		if int64(reader.Len()) > maxBytes {
			return nil, ErrFileTooLarge
		}
		return reader.Bytes(), nil
	}

	if _, err := reader.ReadFrom(file); err != nil {
		return nil, fmt.Errorf("read uploaded image: %w", err)
	}

	return reader.Bytes(), nil
}

func decodeImage(sourceBytes []byte, contentType string) (image.Image, error) {
	reader := bytes.NewReader(sourceBytes)

	switch contentType {
	case "image/jpeg":
		img, err := jpeg.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("decode jpeg image: %w", err)
		}
		return img, nil
	case "image/png":
		img, err := png.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("decode png image: %w", err)
		}
		return img, nil
	case "image/gif":
		img, err := gif.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("decode gif image: %w", err)
		}
		return img, nil
	case "image/webp":
		img, err := webp.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("decode webp image: %w", err)
		}
		return img, nil
	default:
		return nil, ErrUnsupportedFormat
	}
}

func resizeToFit(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}

	if maxWidth <= 0 {
		maxWidth = width
	}
	if maxHeight <= 0 {
		maxHeight = height
	}

	scale := minFloat(1, minFloat(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height)))
	if scale >= 1 {
		dst := image.NewNRGBA(image.Rect(0, 0, width, height))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)
		return dst
	}

	targetWidth := max(1, int(float64(width)*scale))
	targetHeight := max(1, int(float64(height)*scale))
	dst := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)
	return dst
}

func shouldEncodeAsPNG(contentType string, img image.Image) bool {
	switch contentType {
	case "image/png", "image/gif":
		return true
	case "image/webp":
		return hasTransparency(img)
	default:
		return hasTransparency(img)
	}
}

func hasTransparency(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha != 0xffff {
				return true
			}
		}
	}

	return false
}

func encodeImage(destinationPath string, img image.Image, extension string, jpegQuality int) error {
	outputFile, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("create destination image: %w", err)
	}
	defer outputFile.Close()

	switch extension {
	case ".png":
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(outputFile, img); err != nil {
			return fmt.Errorf("encode png image: %w", err)
		}
		return nil
	default:
		if jpegQuality <= 0 {
			jpegQuality = 86
		}
		if err := jpeg.Encode(outputFile, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return fmt.Errorf("encode jpeg image: %w", err)
		}
		return nil
	}
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
