package dsp

import (
	"math"

	"gonum.org/v1/gonum/dsp/fourier"
)

// Chromaprint-compatible audio fingerprint generation.
// This implementation matches the reference C++ Chromaprint library.
//
// Pipeline: Audio → Resample 11025Hz → Hamming+FFT → Energy spectrum →
//   Chroma → Normalize → ChromaFilter → IntegralImage → Classifiers → Subfingerprints
//
// Reference: https://github.com/acoustid/chromaprint (MIT license)

const (
	// Chromaprint standard parameters (algorithm 2 / default)
	chromaprintSampleRate = 11025
	chromaprintFrameSize  = 4096
	chromaprintHopSize    = 4096 / 3 // = 1365 (frame_size - overlap where overlap = frame_size - frame_size/3)
	chromaprintMaxDuration = 120     // seconds
)

// classifierDef defines a Chromaprint classifier with filter parameters and quantizer thresholds.
type classifierDef struct {
	filterType int
	y          int // chroma bin start
	height     int // chroma bin count
	width      int // temporal width (frames)
	t0, t1, t2 float64 // quantizer thresholds
}

// Algorithm 2 classifiers (default for fpcalc).
// Extracted from Chromaprint source: fingerprinter_configuration.cpp kClassifiersTest2
var classifiersAlgo2 = [16]classifierDef{
	{0, 4, 3, 15, 1.98215, 2.35817, 2.63523},
	{4, 4, 6, 15, -1.03809, -0.651211, -0.282167},
	{1, 0, 4, 16, -0.298702, 0.119262, 0.558497},
	{3, 8, 2, 12, -0.105439, 0.0153946, 0.135898},
	{3, 4, 4, 8, -0.142891, 0.0258736, 0.200632},
	{4, 0, 3, 5, -0.826319, -0.590612, -0.368214},
	{1, 2, 2, 9, -0.557409, -0.233035, 0.0534525},
	{2, 7, 3, 4, -0.0646826, 0.00620476, 0.0784847},
	{2, 6, 2, 16, -0.192387, -0.029699, 0.215855},
	{2, 1, 3, 2, -0.0397818, -0.00568076, 0.0292026},
	{5, 10, 1, 15, -0.53823, -0.369934, -0.190235},
	{3, 6, 2, 10, -0.124877, 0.0296483, 0.139239},
	{2, 1, 1, 14, -0.101475, 0.0225617, 0.231971},
	{3, 5, 6, 4, -0.0799915, -0.00729616, 0.063262},
	{1, 9, 2, 12, -0.272556, 0.019424, 0.302559},
	{3, 4, 2, 14, -0.164292, -0.0321188, 0.0846339},
}

// grayCode maps quantizer output (0-3) to gray code values.
var grayCode = [4]int{0, 1, 3, 2}

func quantize(value, t0, t1, t2 float64) int {
	if value < t1 {
		if value < t0 {
			return 0
		}
		return 1
	}
	if value < t2 {
		return 2
	}
	return 3
}

func subtractLog(a, b float64) float64 {
	return math.Log((1.0 + a) / (1.0 + b))
}

// rollingIntegralImage is a 2D cumulative sum table matching Chromaprint's RollingIntegralImage.
type rollingIntegralImage struct {
	data       []float64
	numColumns int
	numRows    int
	maxRows    int
}

func newRollingIntegralImage(maxRows int) *rollingIntegralImage {
	return &rollingIntegralImage{
		maxRows: maxRows + 1,
	}
}

func (img *rollingIntegralImage) addRow(row []float64) {
	if img.numColumns == 0 {
		img.numColumns = len(row)
		img.data = make([]float64, img.maxRows*img.numColumns)
	}

	// Compute partial sums for the new row
	currentRowStart := (img.numRows % img.maxRows) * img.numColumns
	var cumSum float64
	for i, v := range row {
		cumSum += v
		img.data[currentRowStart+i] = cumSum
	}

	// Add previous row's cumulative sums
	if img.numRows > 0 {
		prevRowStart := ((img.numRows - 1) % img.maxRows) * img.numColumns
		for i := 0; i < img.numColumns; i++ {
			img.data[currentRowStart+i] += img.data[prevRowStart+i]
		}
	}

	img.numRows++
}

func (img *rollingIntegralImage) area(r1, c1, r2, c2 int) float64 {
	if r1 == r2 || c1 == c2 {
		return 0.0
	}

	if r1 == 0 {
		row2 := ((r2 - 1) % img.maxRows) * img.numColumns
		if c1 == 0 {
			return img.data[row2+c2-1]
		}
		return img.data[row2+c2-1] - img.data[row2+c1-1]
	}

	row1 := ((r1 - 1) % img.maxRows) * img.numColumns
	row2 := ((r2 - 1) % img.maxRows) * img.numColumns
	if c1 == 0 {
		return img.data[row2+c2-1] - img.data[row1+c2-1]
	}
	return img.data[row2+c2-1] - img.data[row1+c2-1] - img.data[row2+c1-1] + img.data[row1+c1-1]
}

// applyFilter applies one of the 6 Chromaprint filter types to the integral image.
// Matches filter_utils.h Filter0-Filter5 with SubtractLog comparator.
func applyChromaprintFilter(img *rollingIntegralImage, x int, f classifierDef) float64 {
	y := f.y
	w := f.width
	h := f.height

	switch f.filterType {
	case 0: // Full block vs 0
		a := img.area(x, y, x+w, y+h)
		return subtractLog(a, 0)

	case 1: // Top half vs bottom half (horizontal split)
		h2 := h / 2
		a := img.area(x, y+h2, x+w, y+h)
		b := img.area(x, y, x+w, y+h2)
		return subtractLog(a, b)

	case 2: // Right half vs left half (vertical split)
		w2 := w / 2
		a := img.area(x+w2, y, x+w, y+h)
		b := img.area(x, y, x+w2, y+h)
		return subtractLog(a, b)

	case 3: // Checkerboard pattern
		w2 := w / 2
		h2 := h / 2
		a := img.area(x, y+h2, x+w2, y+h) + img.area(x+w2, y, x+w, y+h2)
		b := img.area(x, y, x+w2, y+h2) + img.area(x+w2, y+h2, x+w, y+h)
		return subtractLog(a, b)

	case 4: // Middle third vs outer thirds (horizontal)
		h3 := h / 3
		a := img.area(x, y+h3, x+w, y+2*h3)
		b := img.area(x, y, x+w, y+h3) + img.area(x, y+2*h3, x+w, y+h)
		return subtractLog(a, b)

	case 5: // Middle third vs outer thirds (vertical)
		w3 := w / 3
		a := img.area(x+w3, y, x+2*w3, y+h)
		b := img.area(x, y, x+w3, y+h) + img.area(x+2*w3, y, x+w, y+h)
		return subtractLog(a, b)
	}
	return 0
}

// Fingerprinter generates Chromaprint-compatible audio fingerprints.
type Fingerprinter struct {
	sampleRate int
}

// NewFingerprinter creates a new Fingerprinter.
func NewFingerprinter(sampleRate int) *Fingerprinter {
	return &Fingerprinter{sampleRate: sampleRate}
}

// Fingerprint generates raw subfingerprints from audio samples.
// Input samples should be mono float64 at the original sample rate.
func (f *Fingerprinter) Fingerprint(samples []float64) ([]uint32, error) {
	// Limit to max duration
	maxSamples := f.sampleRate * chromaprintMaxDuration
	if len(samples) > maxSamples {
		samples = samples[:maxSamples]
	}

	// Resample to 11025 Hz
	resampled := Resample(samples, f.sampleRate, chromaprintSampleRate)
	if len(resampled) < chromaprintFrameSize {
		return nil, nil
	}

	// Convert float64 to int16 values matching Chromaprint's int16 input.
	// Chromaprint receives raw int16 samples; we simulate this by rounding
	// to integer values so the window scaling (1/32767) exactly matches.
	int16Samples := make([]float64, len(resampled))
	for i, v := range resampled {
		val := math.Round(v * 32768.0)
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		int16Samples[i] = val
	}

	// Prepare Hamming window with 1/INT16_MAX scaling (matches Chromaprint)
	window := make([]float64, chromaprintFrameSize)
	for i := 0; i < chromaprintFrameSize; i++ {
		window[i] = (0.54 - 0.46*math.Cos(float64(i)*2.0*math.Pi/float64(chromaprintFrameSize-1))) / 32767.0
	}

	// Initialize pipeline components
	chroma := NewChromaExtractor(chromaprintSampleRate, chromaprintFrameSize)
	chromaFilter := NewChromaFilter()

	// Find max filter width for determining when to start calculating fingerprints
	maxFilterWidth := 0
	for _, c := range classifiersAlgo2 {
		if c.width > maxFilterWidth {
			maxFilterWidth = c.width
		}
	}

	integralImage := newRollingIntegralImage(256)
	var subfingerprints []uint32

	// Set up FFT
	fft := fourier.NewFFT(chromaprintFrameSize)

	// Process frames
	numFrames := (len(int16Samples) - chromaprintFrameSize) / chromaprintHopSize + 1

	for frameIdx := 0; frameIdx < numFrames; frameIdx++ {
		start := frameIdx * chromaprintHopSize

		// Apply Hamming window
		windowed := make([]float64, chromaprintFrameSize)
		for i := 0; i < chromaprintFrameSize; i++ {
			windowed[i] = int16Samples[start+i] * window[i]
		}

		// FFT → energy spectrum (real² + imag²)
		coeffs := fft.Coefficients(nil, windowed)
		energy := make([]float64, len(coeffs))
		for i, c := range coeffs {
			energy[i] = real(c)*real(c) + imag(c)*imag(c)
		}

		// Chroma extraction (no interpolation for algorithm 2)
		chromaVec := chroma.Extract(energy, false)

		// Apply chroma filter first (Chromaprint pipeline: Chroma → Filter → Normalize)
		filtered, ok := chromaFilter.Process(chromaVec)
		if !ok {
			continue
		}

		// Normalize after filtering (Euclidean norm)
		NormalizeChroma(filtered)

		// Add to integral image
		integralImage.addRow(filtered)

		// Calculate subfingerprint when we have enough rows
		if integralImage.numRows >= maxFilterWidth {
			offset := integralImage.numRows - maxFilterWidth
			var bits uint32
			for _, c := range classifiersAlgo2 {
				value := applyChromaprintFilter(integralImage, offset, c)
				q := quantize(value, c.t0, c.t1, c.t2)
				bits = (bits << 2) | uint32(grayCode[q])
			}
			subfingerprints = append(subfingerprints, bits)
		}
	}

	return subfingerprints, nil
}

// FingerprintEncoded generates an encoded fingerprint string suitable for AcoustID submission.
func (f *Fingerprinter) FingerprintEncoded(samples []float64) (string, error) {
	subfp, err := f.Fingerprint(samples)
	if err != nil {
		return "", err
	}
	return EncodeFingerprint(subfp)
}
