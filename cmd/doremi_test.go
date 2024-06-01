package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_noteToFreq(t *testing.T) {

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
		assert.InDeltaf(t, td.ExpFreq, noteToFrequency(td.Note), 0.01, td.Note)
	}
}

func Test_noteToFreq_octave4_12(t *testing.T) {

	testData := []struct {
		Note    string
		ExpFreq float64
	}{
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
		assert.InDeltaf(t, td.ExpFreq, noteToFrequency(td.Note), 0.01, td.Note)
	}
}
