package dsp

import "math"

const (
	chromaBins    = 12
	chromaMinFreq = 28   // Hz
	chromaMaxFreq = 3520 // Hz (matches Chromaprint source)
)

// ChromaExtractor converts FFT energy frames to 12-bin chroma vectors.
// Matches the Chromaprint reference implementation (chroma.cpp).
type ChromaExtractor struct {
	notes    []int     // FFT bin -> chroma note index (0-11)
	notesFrc []float64 // fractional part of note for interpolation
	minIndex int
	maxIndex int
}

// NewChromaExtractor creates a ChromaExtractor matching Chromaprint's Chroma class.
// sampleRate and fftSize must match the Chromaprint parameters (11025, 4096).
func NewChromaExtractor(sampleRate, fftSize int) *ChromaExtractor {
	c := &ChromaExtractor{
		notes:    make([]int, fftSize),
		notesFrc: make([]float64, fftSize),
	}
	c.prepareNotes(chromaMinFreq, chromaMaxFreq, fftSize, sampleRate)
	return c
}

func freqToOctave(freq float64) float64 {
	// base = 440.0 / 16.0 = 27.5 (matches Chromaprint)
	return math.Log(freq/27.5) / math.Log(2.0)
}

func (c *ChromaExtractor) prepareNotes(minFreq, maxFreq, frameSize, sampleRate int) {
	c.minIndex = max(1, freqToIndex(float64(minFreq), frameSize, sampleRate))
	c.maxIndex = min(frameSize/2, freqToIndex(float64(maxFreq), frameSize, sampleRate))
	for i := c.minIndex; i < c.maxIndex; i++ {
		freq := indexToFreq(i, frameSize, sampleRate)
		octave := freqToOctave(freq)
		note := float64(chromaBins) * (octave - math.Floor(octave))
		c.notes[i] = int(note)
		c.notesFrc[i] = note - float64(int(note))
	}
}

func indexToFreq(i, frameSize, sampleRate int) float64 {
	return float64(i) * float64(sampleRate) / float64(frameSize)
}

func freqToIndex(freq float64, frameSize, sampleRate int) int {
	return int(math.Round(float64(frameSize) * freq / float64(sampleRate)))
}

// Extract computes a 12-bin chroma vector from FFT energy values.
// The energy values should be real²+imag² (power spectrum), matching Chromaprint's FFT output.
// When interpolate is false (default for algorithm 2), energy is assigned to the nearest note.
func (c *ChromaExtractor) Extract(energy []float64, interpolate bool) []float64 {
	features := make([]float64, chromaBins)

	limit := c.maxIndex
	if len(energy) < limit {
		limit = len(energy)
	}

	for i := c.minIndex; i < limit; i++ {
		note := c.notes[i]
		e := energy[i]
		if interpolate {
			note2 := note
			a := 1.0
			if c.notesFrc[i] < 0.5 {
				note2 = (note + chromaBins - 1) % chromaBins
				a = 0.5 + c.notesFrc[i]
			}
			if c.notesFrc[i] > 0.5 {
				note2 = (note + 1) % chromaBins
				a = 1.5 - c.notesFrc[i]
			}
			features[note] += e * a
			features[note2] += e * (1.0 - a)
		} else {
			features[note] += e
		}
	}

	return features
}

// NormalizeChroma applies Euclidean normalization matching Chromaprint's ChromaNormalizer.
// If the norm is below threshold (0.01), all values are set to zero.
func NormalizeChroma(features []float64) {
	var squares float64
	for _, v := range features {
		squares += v * v
	}
	norm := 0.0
	if squares > 0 {
		norm = math.Sqrt(squares)
	}
	if norm < 0.01 {
		for i := range features {
			features[i] = 0
		}
	} else {
		for i := range features {
			features[i] /= norm
		}
	}
}

// ChromaFilter applies the Chromaprint FIR filter [0.25, 0.75, 1.0, 0.75, 0.25]
// to a sequence of chroma vectors, matching ChromaFilter in chroma_filter.cpp.
type ChromaFilter struct {
	coefficients []float64
	buffer       [][]float64
	bufferOffset int
	bufferSize   int
}

// NewChromaFilter creates a ChromaFilter with the standard Chromaprint coefficients.
func NewChromaFilter() *ChromaFilter {
	return &ChromaFilter{
		coefficients: []float64{0.25, 0.75, 1.0, 0.75, 0.25},
		buffer:       make([][]float64, 8),
		bufferOffset: 0,
		bufferSize:   1,
	}
}

// Process feeds a chroma vector into the filter. Returns the filtered result
// and true if output is available, or nil and false if still filling the buffer.
func (f *ChromaFilter) Process(features []float64) ([]float64, bool) {
	f.buffer[f.bufferOffset] = features
	f.bufferOffset = (f.bufferOffset + 1) % 8

	if f.bufferSize >= len(f.coefficients) {
		offset := (f.bufferOffset + 8 - len(f.coefficients)) % 8
		result := make([]float64, chromaBins)
		for i := 0; i < chromaBins; i++ {
			for j := 0; j < len(f.coefficients); j++ {
				result[i] += f.buffer[(offset+j)%8][i] * f.coefficients[j]
			}
		}
		return result, true
	}
	f.bufferSize++
	return nil, false
}
