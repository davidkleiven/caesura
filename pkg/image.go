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
	"math"
	"time"

	"github.com/davidkleiven/caesura/config"
	"github.com/hhrutter/tiff"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

var ErrMultipleImagesPerPage = errors.New("multiple images per page are not supported")
var ErrGrayConversion = errors.New("gray conversion failed")

type ImageSet struct {
	Images  []bytes.Buffer
	Updated time.Time
	Options config.AdaptiveGaussianConfig
}

func (is *ImageSet) DigestImage(img model.Image, hasOneImagePerPage bool, maxPageDigits int) error {
	grayImage, err := ToImageGray(img)
	if err != nil {
		return errors.Join(ErrGrayConversion, fmt.Errorf("failed to convert image %s on page %d to grayscale: %w", img.Name, img.PageNr, err))
	}

	thresholdedImage := GaussianAdaptiveThreshold(grayImage, is.Options.BlockSize, is.Options.ThresholdShift)
	var buf bytes.Buffer
	if err := png.Encode(&buf, thresholdedImage); err != nil {
		return fmt.Errorf("failed to encode image %s on page %d to PNG: %w", img.Name, img.PageNr, err)
	}

	is.Images = append(is.Images, buf)
	is.Updated = time.Now()
	slog.Info("Processed image", "name", img.Name, "page", img.PageNr, "size", buf.Len())
	return nil
}

func ProcessImages(rs io.ReadSeeker, options config.AdaptiveGaussianConfig) (*ImageSet, error) {
	images := ImageSet{Options: options}
	err := api.ExtractImages(rs, []string{}, images.DigestImage, nil)
	return &images, err
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

func GaussianAdaptiveThreshold(img *image.Gray, blockSize int, c int) *image.Paletted {
	Assert(img != nil, "Image must not be nil")
	Assert(blockSize > 0, "Block size must be greater than zero")

	gaussians := PreCalculateGaussians(blockSize)
	slog.Info("Pre-calculated Gaussian weights", "blockSize", blockSize, "numWeights", len(gaussians), "shift", c)
	bounds := img.Bounds()
	thresholdedImg := image.NewPaletted(bounds, color.Palette{color.Black, color.White})

	const (
		black = 0
		white = 1
	)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {

			// Calculate the local mean and standard deviation using the Gaussian weights
			localSum := 0.0
			localWeightSum := 0.0
			for dy := -blockSize / 2; dy <= blockSize/2; dy++ {
				for dx := -blockSize / 2; dx <= blockSize/2; dx++ {
					ny, nx := y+dy, x+dx

					// Use Manhatten distance since this allows for pre-calculation of Gaussian weights
					gaussianWeightPosition := AbsInt(dy) + AbsInt(dx)
					if ny >= bounds.Min.Y && ny < bounds.Max.Y && nx >= bounds.Min.X && nx < bounds.Max.X && gaussianWeightPosition < len(gaussians) {
						weight := gaussians[gaussianWeightPosition]
						localSum += float64(img.GrayAt(nx, ny).Y) * weight
						localWeightSum += weight
					}
				}
			}
			localMean := localSum / Confine(localWeightSum, 1.0, 255.0)
			threshold := localMean - float64(c)
			if float64(img.GrayAt(x, y).Y) < threshold {
				thresholdedImg.SetColorIndex(x, y, black)
			} else {
				thresholdedImg.SetColorIndex(x, y, white)
			}
		}
	}
	return thresholdedImg
}

func PreCalculateGaussians(blockSize int) []float64 {
	minWeight := 0.02 // Ensure that a blocksize of 1 results in one point
	numPoints := int(0.5*float64(blockSize)*math.Sqrt(-math.Log(minWeight))) + 1
	gaussians := make([]float64, numPoints)
	for i := range numPoints {
		gaussians[i] = math.Exp(-math.Pow(2.0*float64(i)/float64(blockSize), 2))
	}
	return gaussians
}
