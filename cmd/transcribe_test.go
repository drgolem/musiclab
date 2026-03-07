package cmd

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateSineSignal(freq float64, sampleRate int, durationMs int) []float64 {
	nSamples := sampleRate * durationMs / 1000
	samples := make([]float64, nSamples)
	for i := range samples {
		samples[i] = 0.8 * math.Sin(2*math.Pi*freq*float64(i)/float64(sampleRate))
	}
	return samples
}

func generateSilence(sampleRate int, durationMs int) []float64 {
	return make([]float64, sampleRate*durationMs/1000)
}

func TestTranscribeAudio_SingleNote(t *testing.T) {
	sr := 44100
	// Generate 500ms of A4 (440 Hz)
	samples := generateSineSignal(440.0, sr, 500)

	notes := transcribeAudio(samples, sr, 4096, 2048, 0.02)

	require.NotEmpty(t, notes)

	// Should detect A4 as the primary note
	foundA4 := false
	for _, n := range notes {
		if n.note == "A4" {
			foundA4 = true
			break
		}
	}
	assert.True(t, foundA4, "should detect A4, got notes: %v", noteNames(notes))
}

func TestTranscribeAudio_TwoNotes(t *testing.T) {
	sr := 44100
	// 400ms of C5, then 400ms of E5
	samples := append(
		generateSineSignal(523.25, sr, 400),
		generateSineSignal(659.25, sr, 400)...,
	)

	notes := transcribeAudio(samples, sr, 4096, 2048, 0.02)

	require.GreaterOrEqual(t, len(notes), 2, "should detect at least 2 notes")

	// First note should be C5, second should be E5
	assert.Equal(t, "C5", notes[0].note, "first note should be C5")
	foundE5 := false
	for _, n := range notes {
		if n.note == "E5" {
			foundE5 = true
		}
	}
	assert.True(t, foundE5, "should detect E5")
}

func TestTranscribeAudio_Silence(t *testing.T) {
	sr := 44100
	samples := generateSilence(sr, 500)

	notes := transcribeAudio(samples, sr, 4096, 2048, 0.02)

	// All notes should be rests
	for _, n := range notes {
		assert.Equal(t, "-", n.note, "silence should produce rest notes")
	}
}

func TestTranscribeAudio_NoteWithSilence(t *testing.T) {
	sr := 44100
	// 300ms of G4, 200ms silence, 300ms of A4
	samples := append(generateSineSignal(392.0, sr, 300), generateSilence(sr, 200)...)
	samples = append(samples, generateSineSignal(440.0, sr, 300)...)

	notes := transcribeAudio(samples, sr, 4096, 2048, 0.02)

	require.GreaterOrEqual(t, len(notes), 3, "should detect at least 3 segments (note, silence, note)")

	// Check we have both G4 and A4 with a rest between them
	noteSeq := noteNames(notes)
	foundG4 := false
	foundRest := false
	foundA4 := false
	for _, n := range noteSeq {
		if n == "G4" {
			foundG4 = true
		}
		if n == "-" && foundG4 {
			foundRest = true
		}
		if n == "A4" && foundRest {
			foundA4 = true
		}
	}
	assert.True(t, foundG4, "should detect G4")
	assert.True(t, foundRest, "should detect silence between notes")
	assert.True(t, foundA4, "should detect A4 after silence")
}

func TestTranscribeAudio_Empty(t *testing.T) {
	notes := transcribeAudio(nil, 44100, 4096, 2048, 0.02)
	assert.Empty(t, notes)
}

func TestTranscribeAudio_DurationAccuracy(t *testing.T) {
	sr := 44100
	// 1000ms of E4
	samples := generateSineSignal(329.63, sr, 1000)

	notes := transcribeAudio(samples, sr, 4096, 2048, 0.02)

	// Find the E4 note and check its duration is roughly 1000ms
	var totalE4Ms int
	for _, n := range notes {
		if n.note == "E4" {
			totalE4Ms += n.durMs
		}
	}
	// Allow ~100ms tolerance for STFT framing
	assert.InDelta(t, 1000, totalE4Ms, 150, "E4 duration should be ~1000ms, got %dms", totalE4Ms)
}

func TestTranscribeAudio_ShortNotesFiltered(t *testing.T) {
	sr := 44100
	// Very short 10ms blip — should be filtered out (< 30ms minimum)
	samples := generateSineSignal(440.0, sr, 10)

	notes := transcribeAudio(samples, sr, 4096, 2048, 0.02)

	// With 10ms of audio and 4096-sample frame, there's not enough data for detection
	// Any detected notes shorter than 30ms should be filtered
	for _, n := range notes {
		assert.GreaterOrEqual(t, n.durMs, 30, "notes shorter than 30ms should be filtered")
	}
}

func TestTranscribeAudio_RepeatedSamePitch_WithGap(t *testing.T) {
	sr := 44100
	// Two A4 notes separated by a 100ms silence gap (clear articulation)
	note1 := generateADSRNote(440.0, sr, 500)
	gap := generateSilence(sr, 100)
	note2 := generateADSRNote(440.0, sr, 500)
	samples := append(note1, gap...)
	samples = append(samples, note2...)

	notes := transcribeAudio(samples, sr, 4096, 2048, 0.02)

	// Should detect 2 A4 segments with silence between them
	a4Count := 0
	for _, n := range notes {
		if n.note == "A4" {
			a4Count++
		}
	}
	assert.GreaterOrEqual(t, a4Count, 2, "should detect 2 separate A4 notes with 100ms gap, got %d from %v", a4Count, noteNames(notes))
}

func generateADSRNote(freq float64, sampleRate int, durationMs int) []float64 {
	nSamples := sampleRate * durationMs / 1000
	samples := make([]float64, nSamples)
	durSec := float64(durationMs) / 1000.0
	attack := 0.1 * durSec
	decay := 0.3 * durSec
	release := 0.3 * durSec
	sustain := 0.56 // 0.8 * 0.7

	step := freq / float64(sampleRate)
	var phase float64
	for i := range samples {
		t := float64(i) / float64(sampleRate)
		var amp float64
		if t < attack {
			amp = 0.7 * t / attack
		} else if t < attack+decay {
			amp = 0.7 + (sustain-0.7)*(t-attack)/decay
		} else if t < durSec-release {
			amp = sustain
		} else {
			amp = sustain * (durSec - t) / release
		}
		samples[i] = amp * math.Sin(2*math.Pi*phase)
		phase += step
		_, phase = math.Modf(phase)
	}
	return samples
}

func noteNames(notes []detectedNote) []string {
	names := make([]string, len(notes))
	for i, n := range notes {
		names[i] = n.note
	}
	return names
}
