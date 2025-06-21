package pkg

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hhrutter/tiff"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func ninePixelImage() []byte {
	return []byte{
		0x00, 0x00, 0x00,
		0x00, 0x00, 0xFF,
		0x00, 0x00, 0x00,
	}
}

func pdfWithNinePixelImage(name string, format string) (*os.File, error) {
	pixels := ninePixelImage()

	image := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := range 3 {
		for x := range 3 {
			value := pixels[y*3+x]
			image.Set(x, y, color.RGBA{R: value, G: value, B: value, A: 0xFF})
		}
	}
	// Write to a temporary file
	tmp := filepath.Join(os.TempDir(), name+"."+format)
	tmpFile, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	switch format {
	case "png":
		png.Encode(tmpFile, image)
	case "jpg", "jpeg":
		jpeg.Encode(tmpFile, image, &jpeg.Options{Quality: 100})
	case "tiff":
		tiff.Encode(tmpFile, image, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
	}
	tmpFile.Close()

	outfile := filepath.Join(os.TempDir(), name+"-out.pdf")
	if err := api.ImportImagesFile([]string{tmp}, outfile, nil, nil); err != nil {
		return nil, err
	}

	f, err := os.Open(outfile)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func TestExtractGrayscaleImageFromFile(t *testing.T) {

	for _, format := range []string{"png", "jpg", "jpeg", "tiff"} {
		t.Run("ExtractGrayscaleImage_"+format, func(t *testing.T) {
			pdfFile, err := pdfWithNinePixelImage(strings.Split(t.Name(), "/")[1], format)
			if err != nil {
				t.Error(err)
				return
			}
			defer os.Remove(pdfFile.Name())

			extractedImages, err := api.ExtractImagesRaw(pdfFile, nil, nil)
			if err != nil {
				t.Error(err)
				return
			}

			var img model.Image
			for _, image := range extractedImages[0] {
				img = image
				break
			}

			grayScale, err := ToImageGray(img)
			if err != nil {
				t.Errorf("Failed to convert image to grayscale: %v", err)
				return
			}
			if grayScale.Bounds().Dx() != 3 || grayScale.Bounds().Dy() != 3 {
				t.Errorf("Expected grayscale image size 3x3, got %dx%d", grayScale.Bounds().Dx(), grayScale.Bounds().Dy())
				return
			}

			colors := []color.Gray{grayScale.GrayAt(0, 0), grayScale.GrayAt(2, 1)}
			want := []color.Gray{
				{Y: 0x00}, // Top-left pixel
				{Y: 0xFF}, // Middle pixel
			}

			for i := range want {
				if colors[i] != want[i] {
					t.Errorf("Expected color %v at index %d, got %v", want[i], i, colors[i])
				}
			}
		})
	}
}

func TestToGrayScaleUnsupportedFileType(t *testing.T) {
	img := model.Image{
		FileType: "gif",
	}

	image, err := ToImageGray(img)

	if image.Bounds().Dx() != 0 || image.Bounds().Dy() != 0 {
		t.Errorf("Expected empty image bounds, got %dx%d", image.Bounds().Dx(), image.Bounds().Dy())
	}

	if err == nil || !strings.Contains(err.Error(), "unsupported image file type") {
		t.Errorf("Expected error for unsupported file type, got %v", err)
	}
}

type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, os.ErrInvalid // Simulate a read error
}

func TestToGrayFailingRead(t *testing.T) {
	img := model.Image{
		Reader:   &failingReader{},
		FileType: "jpg",
	}

	image, err := ToImageGray(img)
	if image.Bounds().Dx() != 0 || image.Bounds().Dy() != 0 {
		t.Errorf("Expected empty image bounds, got %dx%d", image.Bounds().Dx(), image.Bounds().Dy())
	}
	if !errors.Is(err, os.ErrInvalid) {
		t.Errorf("Expected read error, got %v", err)
	}
}

func TestDecodeAnEmptyTiff(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	buffer := bytes.NewBuffer([]byte{})

	if err := tiff.Encode(buffer, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true}); err != nil {
		t.Fatalf("Failed to encode empty TIFF: %v", err)
	}

	pdfCpuImg := model.Image{
		Reader:   bytes.NewReader(buffer.Bytes()),
		FileType: "tiff",
	}

	decodedImage, err := ToImageGray(pdfCpuImg)

	if decodedImage.Bounds().Dx() != 3 || decodedImage.Bounds().Dy() != 3 {
		t.Errorf("Expected decoded image size 3x3, got %dx%d", decodedImage.Bounds().Dx(), decodedImage.Bounds().Dy())
	}

	if err != nil {
		t.Fatalf("Failed to decode empty TIFF: %v", err)
	}
}
