package dsp

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResample_SameRate(t *testing.T) {
	samples := []float64{1, 2, 3, 4, 5}
	result := Resample(samples, 44100, 44100)
	assert.Equal(t, samples, result)
}

func TestResample_IntegerDecimation(t *testing.T) {
	// 44100 -> 11025 = 4:1 decimation with sinc resampler
	samples := make([]float64, 44100)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * 100 * float64(i) / 44100)
	}
	result := Resample(samples, 44100, 11025)
	// Sinc resampler may produce slightly fewer samples due to filter delay
	assert.InDelta(t, 11025, len(result), 20, "output length should be ~11025")
	assert.Greater(t, len(result), 11000)
}

func TestResample_PreservesSineFrequency(t *testing.T) {
	const (
		fromRate = 44100
		toRate   = 11025
		freq     = 100.0
		duration = 1.0
	)

	nSamples := int(fromRate * duration)
	samples := make([]float64, nSamples)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / fromRate)
	}

	result := Resample(samples, fromRate, toRate)
	assert.Greater(t, len(result), 11000)

	// Count zero crossings — skip first 50 samples (filter delay)
	crossings := 0
	for i := 51; i < len(result); i++ {
		if (result[i-1] >= 0 && result[i] < 0) || (result[i-1] < 0 && result[i] >= 0) {
			crossings++
		}
	}
	// Expected: ~200 crossings (100 Hz * 2 crossings per cycle * ~1 second)
	assert.InDelta(t, 200, crossings, 10, "zero crossings should correspond to ~100Hz")
}

func TestResample_NonIntegerRatio(t *testing.T) {
	// Non-integer ratio: 48000 -> 11025
	samples := make([]float64, 48000)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * 100 * float64(i) / 48000)
	}
	result := Resample(samples, 48000, 11025)
	expectedLen := int(float64(48000) * float64(11025) / float64(48000))
	assert.InDelta(t, expectedLen, len(result), 20)
}

func TestResample_Empty(t *testing.T) {
	result := Resample(nil, 44100, 11025)
	assert.Empty(t, result)
}
