package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ChromaClose(t *testing.T) {
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
		assert.Equal(t, td.ExpNote, freqToNote(td.Freq),
			fmt.Sprintf("expect %0.2f => %s", td.Freq, td.ExpNote))
	}
}
