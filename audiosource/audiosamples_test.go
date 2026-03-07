package audiosource

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/youpy/go-wav"
)

func TestAudioSamplesFromFile_WAV(t *testing.T) {
	// Generate a short 440Hz sine wave WAV file as a fixture
	const (
		sampleRate = 44100
		channels   = 2
		bits       = 16
		freq       = 440.0
		duration   = 0.1 // seconds
		amplitude  = 0.5
	)

	nSamples := int(sampleRate * duration)
	audioData := make([]byte, nSamples*channels*(bits/8))

	for i := 0; i < nSamples; i++ {
		sample := int16(amplitude * math.Sin(2*math.Pi*freq*float64(i)/sampleRate) * 0x7FFF)
		offset := i * channels * (bits / 8)
		// left channel
		audioData[offset] = byte(sample)
		audioData[offset+1] = byte(sample >> 8)
		// right channel
		audioData[offset+2] = byte(sample)
		audioData[offset+3] = byte(sample >> 8)
	}

	tmpDir := t.TempDir()
	wavPath := filepath.Join(tmpDir, "test_440hz.wav")

	f, err := os.Create(wavPath)
	require.NoError(t, err)

	wavWriter := wav.NewWriter(f,
		uint32(nSamples),
		uint16(channels),
		uint32(sampleRate),
		uint16(bits))

	_, err = wavWriter.Write(audioData)
	require.NoError(t, err)
	f.Close()

	// Test AudioSamplesFromFile
	result, err := AudioSamplesFromFile(wavPath)
	require.NoError(t, err)

	assert.Equal(t, sampleRate, result.SampleRate)
	assert.Greater(t, len(result.Audio), 0)

	// Mono output should have nSamples samples (stereo mixed to mono)
	assert.Equal(t, nSamples, len(result.Audio))

	// All samples should be in [-1.0, 1.0]
	for i, s := range result.Audio {
		assert.LessOrEqual(t, math.Abs(s), 1.0, "sample %d out of range: %f", i, s)
	}

	// Find the peak frequency using a simple zero-crossing counter
	zeroCrossings := 0
	for i := 1; i < len(result.Audio); i++ {
		if (result.Audio[i-1] >= 0 && result.Audio[i] < 0) ||
			(result.Audio[i-1] < 0 && result.Audio[i] >= 0) {
			zeroCrossings++
		}
	}
	// Approximate frequency from zero crossings: freq ≈ zeroCrossings / (2 * duration)
	estimatedFreq := float64(zeroCrossings) / (2 * duration)
	assert.InDelta(t, freq, estimatedFreq, 20.0, "estimated frequency should be close to 440Hz")
}

func TestAudioSamplesFromFile_NotFound(t *testing.T) {
	_, err := AudioSamplesFromFile("/nonexistent/path/file.wav")
	assert.Error(t, err)
}
