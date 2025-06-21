package pkg

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"time"

	"github.com/hhrutter/tiff"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

var ErrMultipleImagesPerPage = errors.New("multiple images per page are not supported")
var ErrGrayConversion = errors.New("gray conversion failed")

type ImageSet struct {
	Images  []bytes.Buffer
	Updated time.Time
}

func (is *ImageSet) DigestImage(img model.Image, hasOneImagePerPage bool, maxPageDigits int) error {
	if !hasOneImagePerPage {
		return errors.Join(ErrMultipleImagesPerPage, fmt.Errorf("page %d has multiple images", img.PageNr))
	}
	return nil
}

func ProcessImages(rs io.ReadSeeker, writer io.Writer) *ImageSet {
	images := ImageSet{}
	api.ExtractImages(rs, []string{}, images.DigestImage, nil)
	// This function is a placeholder for image processing logic.
	// It reads images from the reader and writes processed images to the writer.
	// The actual implementation would depend on the specific requirements of the image processing task.
	return &images
}

func ToImageGray(img model.Image) (*image.Gray, error) {
	slog.Info("Converting image to grayscale", "name", img.Name, "bpc", img.Bpc, "file-type", img.FileType, "color-space", img.Cs)

	var (
		decodedImage image.Image
		readError    error
	)
	switch img.FileType {
	case "jpg", "jpeg":
		decodedImage, readError = jpeg.Decode(img)
	case "png":
		decodedImage, readError = png.Decode(img)
	case "tiff":
		decodedImage, readError = tiff.Decode(img)
	default:
		return &image.Gray{}, fmt.Errorf("unsupported image file type: %s", img.FileType)
	}

	if readError != nil {
		return &image.Gray{}, readError
	}

	bounds := decodedImage.Bounds()
	grayImage := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			grayImage.Set(x, y, color.GrayModel.Convert(decodedImage.At(x, y)))
		}
	}
	return grayImage, nil
}
