package dsp

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/stretchr/testify/assert"
)

// generateSineWave creates a sine wave signal at the given frequency and sample rate.
func generateSineWave(freq float64, sampleRate int, numSamples int) []float64 {
	signal := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		signal[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
	}
	return signal
}

func TestSTFT_NumFrames(t *testing.T) {
	tests := []struct {
		name       string
		frameShift int
		frameLen   int
		inputLen   int
		expected   int
	}{
		{"exact fit", 512, 1024, 1024, 1},
		{"multiple frames", 512, 1024, 2048, 3},
		{"single sample overlap", 512, 1024, 1535, 1},
		{"large input", 512, 1024, 22050, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.frameShift, tt.frameLen)
			input := make([]float64, tt.inputLen)
			assert.Equal(t, tt.expected, s.NumFrames(input))
		})
	}
}

func TestSTFT_DivideFrames(t *testing.T) {
	frameLen := 1024
	frameShift := 512
	s := New(frameShift, frameLen)

	// Create input of known length
	inputLen := 4096
	input := make([]float64, inputLen)
	for i := range input {
		input[i] = float64(i)
	}

	frames := s.DivideFrames(input)

	expectedNumFrames := s.NumFrames(input)
	assert.Equal(t, expectedNumFrames, len(frames), "number of frames should match NumFrames")

	for i, frame := range frames {
		assert.Equal(t, frameLen, len(frame), "frame %d should have length %d", i, frameLen)

		// Verify the frame starts at the correct offset
		expectedStart := float64(i * frameShift)
		assert.Equal(t, expectedStart, frame[0], "frame %d should start at sample %v", i, expectedStart)
	}
}

func TestSTFT_SineWavePeakFrequency(t *testing.T) {
	sampleRate := 44100
	freq := 440.0
	duration := 0.5
	numSamples := int(float64(sampleRate) * duration)

	signal := generateSineWave(freq, sampleRate, numSamples)

	frameLen := 4096
	frameShift := 2048
	s := New(frameShift, frameLen)

	spectrogram := s.STFT(signal)
	assert.Greater(t, len(spectrogram), 0, "spectrogram should have at least one frame")

	// Check the first frame: find the bin with maximum amplitude
	frame := spectrogram[0]
	maxAmp := 0.0
	maxBin := 0
	for i, val := range frame {
		amp := cmplx.Abs(val)
		if amp > maxAmp {
			maxAmp = amp
			maxBin = i
		}
	}

	// The frequency resolution is sampleRate / frameLen
	// Bin index maps to frequency: binIndex * sampleRate / frameLen
	freqResolution := float64(sampleRate) / float64(frameLen)
	peakFreq := float64(maxBin) * freqResolution

	assert.InDelta(t, freq, peakFreq, freqResolution,
		"peak frequency should be within one bin of 440 Hz")
}

func TestSplitSpectrum(t *testing.T) {
	spec := []complex128{
		complex(3.0, 4.0),
		complex(1.0, 0.0),
		complex(0.0, 1.0),
		complex(-1.0, 0.0),
	}

	amp, phase := SplitSpectrum(spec)

	assert.Equal(t, len(spec), len(amp))
	assert.Equal(t, len(spec), len(phase))

	// |3+4i| = 5
	assert.InDelta(t, 5.0, amp[0], 1e-9)
	// |1+0i| = 1
	assert.InDelta(t, 1.0, amp[1], 1e-9)
	// |0+1i| = 1
	assert.InDelta(t, 1.0, amp[2], 1e-9)
	// |-1+0i| = 1
	assert.InDelta(t, 1.0, amp[3], 1e-9)

	// phase of 3+4i = atan2(4, 3)
	assert.InDelta(t, math.Atan2(4, 3), phase[0], 1e-9)
	// phase of 1+0i = 0
	assert.InDelta(t, 0.0, phase[1], 1e-9)
	// phase of 0+1i = pi/2
	assert.InDelta(t, math.Pi/2, phase[2], 1e-9)
	// phase of -1+0i = pi
	assert.InDelta(t, math.Pi, phase[3], 1e-9)
}

func TestSplitSpectrogram(t *testing.T) {
	spectrogram := [][]complex128{
		{complex(1, 0), complex(0, 1)},
		{complex(3, 4), complex(-1, 0)},
	}

	amp, phase := SplitSpectrogram(spectrogram)

	assert.Equal(t, 2, len(amp))
	assert.Equal(t, 2, len(phase))
	assert.Equal(t, 2, len(amp[0]))
	assert.Equal(t, 2, len(amp[1]))

	assert.InDelta(t, 1.0, amp[0][0], 1e-9)
	assert.InDelta(t, 1.0, amp[0][1], 1e-9)
	assert.InDelta(t, 5.0, amp[1][0], 1e-9)
	assert.InDelta(t, 1.0, amp[1][1], 1e-9)
}
