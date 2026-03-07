package dsp

import (
	"math"
	"strconv"
)

// NoteToFrequency converts a note string (e.g. "A4", "C#5", "Eb4") to its frequency in Hz.
// Uses A4=440Hz as the reference.
func NoteToFrequency(note string) float64 {
	refFreq := 440.0
	refOctave := 4

	noteLen := len(note)

	if noteLen == 0 || noteLen > 3 {
		return refFreq
	}

	octave := refOctave

	if noteLen > 1 {
		oct, err := strconv.Atoi(note[noteLen-1:])
		if err == nil {
			octave = oct
		}
	}

	notes := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

	notePart := note[0:1]

	idx := 0
	for _, nt := range notes {
		if notePart == nt {
			break
		}
		idx++
	}

	if noteLen > 1 {
		noteStep := note[1:2]
		if noteStep == "#" {
			idx++
		} else if noteStep == "b" {
			idx--
		}
	}

	refNoteIdx := 9

	octaveDiff := octave - refOctave

	freq := refFreq * math.Pow(2, float64(octaveDiff)+float64(idx-refNoteIdx)/12)

	return freq
}

// FreqToNote converts a frequency to the closest note name (without octave).
func FreqToNote(freq float64) string {
	if freq < 1.0 {
		return ""
	}

	notes := []string{"B3", "C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B", "C5"}

	c4 := NoteToFrequency("C4")
	b4 := NoteToFrequency("B4")

	prec := 15.55

	for {
		if freq <= b4+prec {
			break
		}
		freq = freq / 2
	}

	for {
		if freq >= c4-prec {
			break
		}
		freq = freq * 2
	}

	idx := 0
	for idx < len(notes)-1 {
		nl := NoteToFrequency(notes[idx])
		nr := NoteToFrequency(notes[idx+1])
		if freq >= nl && freq < nr {
			diff := math.Abs(nr-nl) / 2
			if freq < nl+diff {
				return notes[idx]
			} else {
				return notes[idx+1]
			}
		}
		idx++
	}

	return "C4"
}

// FreqToNoteOctave converts a frequency to the closest note name with octave (e.g. "A4", "C#5").
// Returns empty string for frequencies below ~16 Hz.
func FreqToNoteOctave(freq float64) string {
	if freq < 16.0 {
		return ""
	}

	noteNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

	// Semitones from A4 (440 Hz)
	semitones := 12 * math.Log2(freq/440.0)
	// Round to nearest semitone
	semi := int(math.Round(semitones))
	// A4 is index 9 in octave 4
	// absolute note index from C0
	noteIdx := semi + 9 + 4*12
	if noteIdx < 0 {
		return ""
	}

	octave := noteIdx / 12
	note := noteIdx % 12

	return noteNames[note] + strconv.Itoa(octave)
}

// PeaksHash computes a hash from peak frequency values across octave bins.
func PeaksHash(peaks []float64) uint64 {
	h := uint64(0)
	idx := len(peaks) - 1
	for idx >= 0 {
		v := uint64(int64(peaks[idx]) & ^1)
		h *= 1000
		h += v
		idx--
	}

	return h
}

// OctaveBinPeaks finds peak frequencies in octave-separated bins from a spectrum.
func OctaveBinPeaks(sampleRate int, minAmpl float64, spectr []float64) []float64 {
	binNotesNames := []string{
		"C2", "C3", "C4", "C5", "C6", "C7", "C8",
	}

	binNotes := make([]float64, 0, len(binNotesNames))

	for _, nt := range binNotesNames {
		freq := NoteToFrequency(nt)
		binNotes = append(binNotes, freq)
	}

	freqToBinIndex := func(freq float64) int {
		idx := 0
		for idx < len(binNotes) && binNotes[idx] < freq {
			idx++
		}
		return idx
	}

	binFreqAmpl := make([]float64, len(binNotes)+1)
	binPeakFreq := make([]float64, len(binNotes)+1)

	for i, m := range spectr {
		freq := float64(i) * float64(sampleRate) / (2 * float64(len(spectr)))

		if freq < 30.0 || freq > 5000.0 {
			continue
		}

		if m < minAmpl {
			continue
		}

		idx := freqToBinIndex(freq)
		if binFreqAmpl[idx] < m {
			binFreqAmpl[idx] = m
			binPeakFreq[idx] = freq
		}
	}
	return binPeakFreq
}
