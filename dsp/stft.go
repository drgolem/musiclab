package dsp

import (
	"math"
	"math/cmplx"
	"slices"

	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/gonum/dsp/window"
)

type STFT struct {
	FrameShift int
	FrameLen   int
	Window     func([]float64) []float64 // window function
}

// New returns a new STFT instance.
func New(frameShift, frameLen int) *STFT {
	s := &STFT{
		FrameShift: frameShift,
		FrameLen:   frameLen,
		Window:     window.Hann,
	}

	return s
}

// NumFrames returnrs the number of frames that will be analyzed in STFT.
func (s *STFT) NumFrames(input []float64) int {
	return int(float64(len(input)-s.FrameLen)/float64(s.FrameShift)) + 1
}

// DivideFrames returns overlapping divided frames for STFT.
func (s *STFT) DivideFrames(input []float64) [][]float64 {
	numFrames := s.NumFrames(input)
	frames := make([][]float64, numFrames)
	for i := 0; i < numFrames; i++ {
		frames[i] = s.FrameAt(input, i)
	}
	return frames
}

// FrameAt returns frame at specified index given an input signal.
// Note that it doesn't make copy of input.
func (s *STFT) FrameAt(input []float64, index int) []float64 {
	return input[index*s.FrameShift : index*s.FrameShift+s.FrameLen]
}

// STFT returns complex spectrogram given an input signal.
func (s *STFT) STFT(input []float64) [][]complex128 {
	numFrames := s.NumFrames(input)
	spectrogram := make([][]complex128, numFrames)

	fft := fourier.NewFFT(s.FrameLen)

	frames := s.DivideFrames(input)
	for i, frame := range frames {
		// Windowing
		frameCopy := slices.Clone(frame)
		windowed := s.Window(frameCopy)
		// Complex Spectrum
		coeff := fft.Coefficients(nil, windowed)

		spectrogram[i] = coeff
	}

	return spectrogram
}

// SplitSpectrum splits complex spectrum X(k) to amplitude |X(k)|
// and angle(X(k))
func SplitSpectrum(spec []complex128) ([]float64, []float64) {
	amp := make([]float64, len(spec))
	phase := make([]float64, len(spec))
	for i, val := range spec {
		amp[i] = cmplx.Abs(val)
		phase[i] = math.Atan2(imag(val), real(val))
	}

	return amp, phase
}

// SplitSpectrogram returns SpilitSpectrum for each time frame.
func SplitSpectrogram(spectrogram [][]complex128) ([][]float64, [][]float64) {
	numFrames, numFreqBins := len(spectrogram), len(spectrogram[0])
	amp := create2DSlice(numFrames, numFreqBins)
	phase := create2DSlice(numFrames, numFreqBins)

	for i := 0; i < numFrames; i++ {
		amp[i], phase[i] = SplitSpectrum(spectrogram[i])
	}

	return amp, phase
}

func create2DSlice(rows, cols int) [][]float64 {
	s := make([][]float64, rows)
	for i := range s {
		s[i] = make([]float64, cols)
	}
	return s
}
