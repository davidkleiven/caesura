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
	"golang.org/x/exp/slog"
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

func TestPreCalculateGaussian(t *testing.T) {
	for _, test := range []struct {
		blockSize int
		want      int
		desc      string
	}{
		{
			blockSize: 1,
			want:      1,
			desc:      "block size 1 should return 1",
		},
		{
			blockSize: 2,
			want:      2,
			desc:      "block size 1 should return 1",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			got := PreCalculateGaussians(test.blockSize)
			if len(got) != test.want {
				t.Errorf("PreCalculateGaussians(%d) = %d, want %d", test.blockSize, len(got), test.want)
			}
		})
	}
}

type point struct {
	x, y int
}

func distributedSevens(sevenPositions []point) *image.Gray {
	const tileSize = 8
	seven := []byte{
		0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF,
		0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}

	img := image.NewGray(image.Rect(0, 0, 256, 256))

	// Fill image with white pixels
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			img.Set(x, y, color.Gray{Y: 0xFF})
		}
	}

	for i := range sevenPositions {
		if sevenPositions[i].x < 0 || sevenPositions[i].y < 0 ||
			sevenPositions[i].x*tileSize+tileSize > img.Bounds().Dx() ||
			sevenPositions[i].y*tileSize+tileSize > img.Bounds().Dy() {
			panic("Seven position out of bounds")
		}

		for y := range tileSize {
			for x := range tileSize {
				value := seven[y*tileSize+x]
				img.Set(sevenPositions[i].x*tileSize+x, sevenPositions[i].y*tileSize+y, color.Gray{Y: value})
			}
		}
	}
	return img
}

func mustSaveImage(img image.Image, name string) func() {
	out, err := os.CreateTemp("", name+"*.png")
	if err != nil {
		panic("Failed to create temp file: " + err.Error())
	}
	if err := png.Encode(out, img); err != nil {
		panic("Failed to save image: " + err.Error())
	}
	slog.Info("Saved image", "name", out.Name())
	return func() {
		os.Remove(out.Name())
	}
}

func TestGaussianAdaptiveThreshold(t *testing.T) {
	sevenPositions := []point{{0, 0}, {2, 0}, {0, 4}, {4, 4}, {16, 14}, {16, 15}, {15, 16}, {15, 15}}

	img := distributedSevens(sevenPositions)
	thresholded := GaussianAdaptiveThreshold(img, 8, 2)

	const saveImg = false
	if saveImg {
		originalCleanup := mustSaveImage(img, "TestGaussianAdaptiveThreshold-original")
		defer originalCleanup()
		thresholdedCleanup := mustSaveImage(thresholded, "TestGaussianAdaptiveThreshold-thresholded")
		defer thresholdedCleanup()
	}

	// For this image adaptive thresholding should be the equivalent with simple thresholding
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			originalColor := img.GrayAt(x, y)
			thresholdedColor := thresholded.ColorIndexAt(x, y)

			if originalColor.Y == 0xFF {
				if thresholdedColor != 1 { // Should be white
					t.Errorf("Expected white at (%d, %d), got %d", x, y, thresholdedColor)
				}
			} else {
				if thresholdedColor != 0 { // Should be black
					t.Errorf("Expected black at (%d, %d), got %d", x, y, thresholdedColor)
				}
			}
		}
	}
}
