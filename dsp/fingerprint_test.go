package dsp

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprinter_ProducesOutput(t *testing.T) {
	// Generate a 5-second sine wave at 440Hz, 44100Hz sample rate
	const (
		sampleRate = 44100
		duration   = 5 // seconds
		freq       = 440.0
	)
	nSamples := sampleRate * duration
	samples := make([]float64, nSamples)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / sampleRate)
	}

	fp := NewFingerprinter(sampleRate)
	subfps, err := fp.Fingerprint(samples)
	require.NoError(t, err)
	assert.Greater(t, len(subfps), 0, "should produce subfingerprints")
}

func TestFingerprinter_ConsistentOutput(t *testing.T) {
	// Same input should produce same output
	const (
		sampleRate = 44100
		duration   = 2
		freq       = 440.0
	)
	nSamples := sampleRate * duration
	samples := make([]float64, nSamples)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / sampleRate)
	}

	fp := NewFingerprinter(sampleRate)
	result1, err := fp.Fingerprint(samples)
	require.NoError(t, err)

	result2, err := fp.Fingerprint(samples)
	require.NoError(t, err)

	assert.Equal(t, result1, result2, "same input should produce same fingerprint")
}

func TestFingerprinter_DifferentPitchClassesDiffer(t *testing.T) {
	const (
		sampleRate = 44100
		duration   = 3
	)
	nSamples := sampleRate * duration

	gen := func(freq float64) []float64 {
		s := make([]float64, nSamples)
		for i := range s {
			s[i] = math.Sin(2 * math.Pi * freq * float64(i) / sampleRate)
		}
		return s
	}

	fp := NewFingerprinter(sampleRate)
	// A4 (440Hz) vs C4 (261.63Hz) - different pitch classes
	fp1, err := fp.Fingerprint(gen(440.0))
	require.NoError(t, err)

	fp2, err := fp.Fingerprint(gen(261.63))
	require.NoError(t, err)

	// Fingerprints should differ for different pitch classes
	assert.NotEqual(t, fp1, fp2, "different pitch classes should produce different fingerprints")
}

func TestFingerprinter_EncodedRoundTrip(t *testing.T) {
	const (
		sampleRate = 44100
		duration   = 5
		freq       = 440.0
	)
	nSamples := sampleRate * duration
	samples := make([]float64, nSamples)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / sampleRate)
	}

	fp := NewFingerprinter(sampleRate)
	subfps, err := fp.Fingerprint(samples)
	require.NoError(t, err)

	encoded, err := EncodeFingerprint(subfps)
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded, err := DecodeFingerprint(encoded)
	require.NoError(t, err)
	assert.Equal(t, subfps, decoded)
}

func TestFingerprinter_TooShortInput(t *testing.T) {
	samples := make([]float64, 100) // way too short for STFT
	fp := NewFingerprinter(44100)
	subfps, err := fp.Fingerprint(samples)
	require.NoError(t, err)
	assert.Nil(t, subfps, "too-short input should produce nil")
}

func TestFingerprinter_LimitsTo120Seconds(t *testing.T) {
	const sampleRate = 44100
	// 200 seconds of audio
	samples := make([]float64, sampleRate*200)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * 440 * float64(i) / sampleRate)
	}

	fp := NewFingerprinter(sampleRate)
	subfps, err := fp.Fingerprint(samples)
	require.NoError(t, err)

	// Should produce fingerprints for ~120 seconds, not 200
	// At 11025 Hz with hop 2720, ~120 seconds gives about
	// (120 * 11025 - 4096) / 2720 + 1 ≈ 485 frames
	// At 11025Hz, hop=1365, 120s: ~(120*11025-4096)/1365 ≈ 966 frames, minus chroma filter warmup
	assert.Less(t, len(subfps), 1100, "should limit to ~120 seconds of fingerprints")
}
