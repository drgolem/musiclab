package dsp

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoteToFrequency(t *testing.T) {
	testData := []struct {
		Note    string
		ExpFreq float64
	}{
		{"C4", 261.63},
		{"C5", 523.25},
		{"C3", 130.81},
		{"A4", 440.0},
		{"A5", 880.0},
		{"A3", 220.0},
		{"G4", 392.0},
		{"G5", 783.99},
		{"G3", 196.00},
	}

	for _, td := range testData {
		t.Run(td.Note, func(t *testing.T) {
			assert.InDeltaf(t, td.ExpFreq, NoteToFrequency(td.Note), 0.01, td.Note)
		})
	}
}

func TestNoteToFrequency_Octave4_AllNotes(t *testing.T) {
	testData := []struct {
		Note    string
		ExpFreq float64
	}{
		{"C#", 277.18},
		{"C", 261.63},
		{"C4", 261.63},
		{"C#4", 277.18},
		{"Db4", 277.18},
		{"D", 293.66},
		{"D4", 293.66},
		{"D#4", 311.13},
		{"Eb4", 311.13},
		{"E4", 329.63},
		{"E#4", 349.23},
		{"Fb4", 329.63},
		{"F4", 349.23},
		{"F#4", 369.99},
		{"Gb4", 369.99},
		{"G4", 392.00},
		{"G#4", 415.3},
		{"Ab4", 415.3},
		{"A4", 440.0},
		{"A#4", 466.16},
		{"Bb4", 466.16},
		{"B4", 493.88},
		{"Cb5", 493.88},
		{"B#4", 523.25},
	}

	for _, td := range testData {
		t.Run(td.Note, func(t *testing.T) {
			assert.InDeltaf(t, td.ExpFreq, NoteToFrequency(td.Note), 0.01, td.Note)
		})
	}
}

func TestNoteToFrequency_MinDiff(t *testing.T) {
	testData := []struct {
		Note    string
		ExpFreq float64
	}{
		{"C4", 261.63},
		{"C#4", 277.18},
		{"D4", 293.66},
		{"D#4", 311.13},
		{"E4", 329.63},
		{"F4", 349.23},
		{"F#4", 369.99},
		{"G4", 392.00},
		{"G#4", 415.3},
		{"A4", 440.0},
		{"A#4", 466.16},
		{"B4", 493.88},
	}

	min := math.MaxFloat64
	prev := float64(0)
	for idx, td := range testData {
		freq := NoteToFrequency(td.Note)
		assert.InDeltaf(t, td.ExpFreq, freq, 0.01, td.Note)

		if idx >= 1 && freq != prev {
			mn := math.Abs(freq - prev)
			if mn < min {
				min = mn
			}
		}
		prev = freq
	}
	assert.InDeltaf(t, 15.55, min, 0.01, "min freq diff between adjacent notes in octave 4")
}

func TestFreqToNote(t *testing.T) {
	testData := []struct {
		ExpNote string
		Freq    float64
	}{
		{"C", 269.4},
		{"C#", 269.6},
		{"C", 261.00},
		{"B", 494.88},
		{"B", 493.88},
		{"B", 491.88},
		{"A#", 466.16},
		{"A#", 467.16},
		{"A#", 464.16},
		{"A", 440.0},
		{"A", 442.0},
		{"A", 439.0},
		{"G#", 415.3},
		{"G#", 416.3},
		{"G#", 414.3},
		{"G", 392.00},
		{"G", 391.00},
		{"G", 393.00},
		{"F#", 369.99},
		{"F#", 370.99},
		{"F#", 368.00},
		{"F", 349.23},
		{"F", 351.00},
		{"F", 348.00},
		{"E", 329.63},
		{"E", 329.00},
		{"E", 331.00},
		{"D#", 311.13},
		{"D#", 310.13},
		{"D", 294.66},
		{"D", 293.66},
		{"C#", 277.18},
		{"C", 261.63},
		{"C", 262.00},
	}

	for _, td := range testData {
		t.Run(fmt.Sprintf("%s_%.2fHz", td.ExpNote, td.Freq), func(t *testing.T) {
			assert.Equal(t, td.ExpNote, FreqToNote(td.Freq),
				fmt.Sprintf("expect %0.2f => %s", td.Freq, td.ExpNote))
		})
	}
}

func TestFreqToNote_OtherOctaves(t *testing.T) {
	// A note at 880 Hz (A5) should still map to "A"
	assert.Equal(t, "A", FreqToNote(880.0))
	// 220 Hz (A3)
	assert.Equal(t, "A", FreqToNote(220.0))
}

func TestFreqToNote_InvalidFreq(t *testing.T) {
	assert.Equal(t, "", FreqToNote(0.0))
	assert.Equal(t, "", FreqToNote(-1.0))
	assert.Equal(t, "", FreqToNote(0.5))
}

func TestPeaksHash(t *testing.T) {
	peaks := []float64{1, 2, 3, 4}

	h := PeaksHash(peaks)

	assert.Equal(t, uint64(4002002000), h)
}

func TestPeaksHash_Empty(t *testing.T) {
	peaks := []float64{}
	h := PeaksHash(peaks)
	assert.Equal(t, uint64(0), h)
}

func TestPeaksHash_SingleElement(t *testing.T) {
	peaks := []float64{440}
	h := PeaksHash(peaks)
	assert.Equal(t, uint64(440), h)
}

func TestOctaveBinPeaks(t *testing.T) {
	sampleRate := 44100
	// Create a synthetic spectrum with a known peak at ~440 Hz
	// Spectrum length corresponds to half the FFT size
	fftSize := 4096
	spectrLen := fftSize / 2

	spectr := make([]float64, spectrLen)

	// Place a peak at the bin corresponding to ~440 Hz
	// bin index = freq * 2 * spectrLen / sampleRate
	binIndex440 := int(440.0 * 2 * float64(spectrLen) / float64(sampleRate))
	spectr[binIndex440] = 100.0

	// Place another peak at ~261 Hz (C4)
	binIndex261 := int(261.0 * 2 * float64(spectrLen) / float64(sampleRate))
	spectr[binIndex261] = 80.0

	peaks := OctaveBinPeaks(sampleRate, 10.0, spectr)

	// Verify peaks is not empty and has the expected length
	// binNotes has 7 entries, so binPeakFreq has 8 entries
	assert.Equal(t, 8, len(peaks))

	// At least one peak should be close to 440 Hz
	found440 := false
	found261 := false
	for _, p := range peaks {
		if math.Abs(p-440.0) < 20 {
			found440 = true
		}
		if math.Abs(p-261.0) < 20 {
			found261 = true
		}
	}
	assert.True(t, found440, "should find a peak near 440 Hz")
	assert.True(t, found261, "should find a peak near 261 Hz")
}

func TestOctaveBinPeaks_BelowMinAmpl(t *testing.T) {
	sampleRate := 44100
	spectrLen := 2048
	spectr := make([]float64, spectrLen)

	// All values below minAmpl
	for i := range spectr {
		spectr[i] = 1.0
	}

	peaks := OctaveBinPeaks(sampleRate, 10.0, spectr)

	// All peaks should be zero since nothing exceeds minAmpl
	for _, p := range peaks {
		assert.Equal(t, 0.0, p)
	}
}

func TestOctaveBinPeaks_MultiplePeaksInDifferentOctaves(t *testing.T) {
	sampleRate := 44100
	fftSize := 4096
	spectrLen := fftSize / 2
	spectr := make([]float64, spectrLen)

	// Place peaks in different octaves: C3 (~130Hz), A4 (440Hz), E6 (~1318Hz)
	freqs := []float64{130.0, 440.0, 1318.0}
	for _, f := range freqs {
		bin := int(f * 2 * float64(spectrLen) / float64(sampleRate))
		if bin < spectrLen {
			spectr[bin] = 50.0
		}
	}

	peaks := OctaveBinPeaks(sampleRate, 10.0, spectr)
	assert.Equal(t, 8, len(peaks))

	// Each frequency should appear as peak in its respective bin
	foundCount := 0
	for _, p := range peaks {
		if p > 0 {
			foundCount++
		}
	}
	assert.Equal(t, 3, foundCount, "should find exactly 3 peaks in different bins")
}

func TestOctaveBinPeaks_PeakBelowRange(t *testing.T) {
	sampleRate := 44100
	fftSize := 4096
	spectrLen := fftSize / 2
	spectr := make([]float64, spectrLen)

	// Place a peak at 20 Hz (below the 30 Hz cutoff)
	bin := int(20.0 * 2 * float64(spectrLen) / float64(sampleRate))
	spectr[bin] = 100.0

	peaks := OctaveBinPeaks(sampleRate, 10.0, spectr)

	for _, p := range peaks {
		assert.Equal(t, 0.0, p, "peak below 30 Hz should be ignored")
	}
}

func TestOctaveBinPeaks_PeakAboveRange(t *testing.T) {
	sampleRate := 44100
	fftSize := 4096
	spectrLen := fftSize / 2
	spectr := make([]float64, spectrLen)

	// Place a peak at 6000 Hz (above the 5000 Hz cutoff)
	bin := int(6000.0 * 2 * float64(spectrLen) / float64(sampleRate))
	if bin < spectrLen {
		spectr[bin] = 100.0
	}

	peaks := OctaveBinPeaks(sampleRate, 10.0, spectr)

	for _, p := range peaks {
		assert.Equal(t, 0.0, p, "peak above 5000 Hz should be ignored")
	}
}

func TestOctaveBinPeaks_LargestPeakWinsInBin(t *testing.T) {
	sampleRate := 44100
	fftSize := 4096
	spectrLen := fftSize / 2
	spectr := make([]float64, spectrLen)

	// Place two peaks in the same octave bin (C4=261 to C5=523)
	// 300 Hz with amplitude 50, 400 Hz with amplitude 90
	bin300 := int(300.0 * 2 * float64(spectrLen) / float64(sampleRate))
	bin400 := int(400.0 * 2 * float64(spectrLen) / float64(sampleRate))
	spectr[bin300] = 50.0
	spectr[bin400] = 90.0

	peaks := OctaveBinPeaks(sampleRate, 10.0, spectr)

	// The peak in the C4-C5 bin should be near 400 Hz (the louder one)
	foundLarger := false
	for _, p := range peaks {
		if math.Abs(p-400.0) < 20 {
			foundLarger = true
		}
	}
	assert.True(t, foundLarger, "should pick the largest amplitude peak (400 Hz) in the bin")
}

func TestOctaveBinPeaks_EmptySpectrum(t *testing.T) {
	peaks := OctaveBinPeaks(44100, 10.0, []float64{})
	assert.Equal(t, 8, len(peaks))
	for _, p := range peaks {
		assert.Equal(t, 0.0, p)
	}
}

func TestNoteToFrequency_EmptyString(t *testing.T) {
	// Empty string returns reference A4=440
	assert.Equal(t, 440.0, NoteToFrequency(""))
}

func TestNoteToFrequency_HighOctaves(t *testing.T) {
	// C7 and C8 should be valid
	assert.InDelta(t, 2093.00, NoteToFrequency("C7"), 0.01)
	assert.InDelta(t, 4186.01, NoteToFrequency("C8"), 0.01)
}

func TestNoteToFrequency_LowOctaves(t *testing.T) {
	assert.InDelta(t, 65.41, NoteToFrequency("C2"), 0.01)
	assert.InDelta(t, 32.70, NoteToFrequency("C1"), 0.01)
}

func TestPeaksHash_TwoPeaks(t *testing.T) {
	// [100, 440]: iterate last→first: h=440&^1=440, h=440*1000+(100&^1)=440100
	peaks := []float64{100, 440}
	h := PeaksHash(peaks)
	assert.Equal(t, uint64(440100), h)
}

func TestPeaksHash_OddValues(t *testing.T) {
	// Odd values get &^1 (clear lowest bit): 441->440, 261->260
	// [441, 261]: iterate last→first: h=261&^1=260, h=260*1000+(441&^1)=260440
	peaks := []float64{441, 261}
	h := PeaksHash(peaks)
	assert.Equal(t, uint64(260440), h)
}

func TestFreqToNote_BoundaryBetweenNotes(t *testing.T) {
	// Exactly halfway between C4 (261.63) and C#4 (277.18) should pick the closer one
	midpoint := (261.63 + 277.18) / 2
	note := FreqToNote(midpoint)
	assert.Contains(t, []string{"C", "C#"}, note, "boundary should resolve to C or C#")
}

func TestFreqToNote_VeryHighFreq(t *testing.T) {
	// 3520 Hz is A7 — should still resolve to "A"
	assert.Equal(t, "A", FreqToNote(3520.0))
}

func TestFreqToNote_VeryLowFreq(t *testing.T) {
	// 55 Hz is A1 — should still resolve to "A"
	assert.Equal(t, "A", FreqToNote(55.0))
}

func TestFreqToNoteOctave(t *testing.T) {
	tests := []struct {
		freq float64
		want string
	}{
		{440.0, "A4"},
		{261.63, "C4"},
		{523.25, "C5"},
		{880.0, "A5"},
		{220.0, "A3"},
		{329.63, "E4"},
		{392.0, "G4"},
		{493.88, "B4"},
		{130.81, "C3"},
		{1046.50, "C6"},
		{2093.0, "C7"},
		{55.0, "A1"},
		{3520.0, "A7"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1fHz=%s", tt.freq, tt.want), func(t *testing.T) {
			got := FreqToNoteOctave(tt.freq)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFreqToNoteOctave_Sharps(t *testing.T) {
	tests := []struct {
		freq float64
		want string
	}{
		{277.18, "C#4"},
		{311.13, "D#4"},
		{369.99, "F#4"},
		{415.30, "G#4"},
		{466.16, "A#4"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FreqToNoteOctave(tt.freq)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFreqToNoteOctave_Invalid(t *testing.T) {
	assert.Equal(t, "", FreqToNoteOctave(0.0))
	assert.Equal(t, "", FreqToNoteOctave(10.0))
	assert.Equal(t, "", FreqToNoteOctave(-1.0))
}

func TestFreqToNoteOctave_RoundTrip(t *testing.T) {
	// NoteToFrequency -> FreqToNoteOctave should return the same note
	notes := []string{"C4", "D4", "E4", "F4", "G4", "A4", "B4",
		"C5", "C3", "A5", "G#4", "F#4", "D#4", "A#4"}
	for _, note := range notes {
		t.Run(note, func(t *testing.T) {
			freq := NoteToFrequency(note)
			got := FreqToNoteOctave(freq)
			assert.Equal(t, note, got)
		})
	}
}
