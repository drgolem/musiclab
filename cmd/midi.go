package cmd

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/drgolem/musiclab/dsp"
	"github.com/sinshu/go-meltysynth/meltysynth"
	"github.com/spf13/cobra"
	"github.com/youpy/go-wav"
)

var midiCmd = &cobra.Command{
	Use:   "midi",
	Short: "Render MIDI or score CSV to WAV using a SoundFont (.sf2)",
	Long: `Render audio using SoundFont instruments for realistic instrument sounds.

Input can be a MIDI file (.mid) or a score CSV file (note,dur format).
Requires a SoundFont (.sf2) file for instrument samples.

Examples:
  musiclab midi --sf2 Real_Nylon_Gtr.sf2 --midi song.mid --out song.wav
  musiclab midi --sf2 Real_Nylon_Gtr.sf2 --score scores/greensleeves.csv --out greensleeves.wav
  musiclab midi --sf2 Real_Nylon_Gtr.sf2 --score scores/stairway_to_heaven.csv --stdout | musictools play -
  musiclab midi --score scores/greensleeves.csv --export-midi greensleeves.mid`,
	Run: doMidiCmd,
}

func init() {
	rootCmd.AddCommand(midiCmd)

	midiCmd.Flags().String("sf2", "", "SoundFont (.sf2) file for instrument rendering")
	midiCmd.Flags().String("midi", "", "input MIDI file (.mid)")
	midiCmd.Flags().String("score", "", "input score CSV file (note,dur format)")
	midiCmd.Flags().String("out", "", "output WAV file")
	midiCmd.Flags().Bool("stdout", false, "write WAV to stdout (for piping)")
	midiCmd.Flags().String("export-midi", "", "export score as MIDI file (no SF2 needed)")
	midiCmd.Flags().Int("tempo", 120, "tempo in BPM (for score CSV conversion)")
	midiCmd.Flags().Int("program", -1, "MIDI program/instrument number (0-127, -1=use SF2 default)")
	midiCmd.Flags().Int("bank", 0, "MIDI bank number")
	midiCmd.Flags().Float64("speed", 1.0, "playback speed for MIDI files (0.5=half speed, 1.0=normal, 2.0=double)")
}

func doMidiCmd(cmd *cobra.Command, args []string) {
	scoreFile, _ := cmd.Flags().GetString("score")
	midiFile, _ := cmd.Flags().GetString("midi")
	sf2File, _ := cmd.Flags().GetString("sf2")
	outFile, _ := cmd.Flags().GetString("out")
	toStdout, _ := cmd.Flags().GetBool("stdout")
	exportMidi, _ := cmd.Flags().GetString("export-midi")
	tempo, _ := cmd.Flags().GetInt("tempo")

	if scoreFile == "" && midiFile == "" {
		slog.Error("--score or --midi is required")
		return
	}

	// Convert score CSV to MIDI notes
	var midiNotes []dsp.MidiNote
	if scoreFile != "" {
		notes, err := scoreToMidiNotes(scoreFile)
		if err != nil {
			slog.Error("failed to parse score", "error", err)
			return
		}
		midiNotes = notes
	}

	// Export MIDI file if requested
	if exportMidi != "" && len(midiNotes) > 0 {
		f, err := os.Create(exportMidi)
		if err != nil {
			slog.Error("failed to create MIDI file", "error", err)
			return
		}
		if err := dsp.WriteMIDIFile(f, midiNotes, tempo); err != nil {
			f.Close()
			slog.Error("failed to write MIDI file", "error", err)
			return
		}
		f.Close()
		slog.Info("Exported MIDI file", "path", exportMidi, "notes", len(midiNotes))
		if sf2File == "" && outFile == "" && !toStdout {
			return // just export, no rendering
		}
	}

	// For rendering, we need SF2
	if sf2File == "" {
		slog.Error("--sf2 is required for audio rendering")
		return
	}

	if !toStdout && outFile == "" {
		// Auto-name from input
		base := "midi_output"
		if scoreFile != "" {
			base = strings.TrimSuffix(filepath.Base(scoreFile), filepath.Ext(scoreFile))
		} else if midiFile != "" {
			base = strings.TrimSuffix(filepath.Base(midiFile), filepath.Ext(midiFile))
		}
		outFile = base + ".wav"
	}

	slog.Info("Loading SoundFont", "path", sf2File)
	sf2Reader, err := os.Open(sf2File)
	if err != nil {
		slog.Error("failed to open SF2", "error", err)
		return
	}
	soundFont, err := meltysynth.NewSoundFont(sf2Reader)
	sf2Reader.Close()
	if err != nil {
		slog.Error("failed to parse SF2", "error", err)
		return
	}

	speed, _ := cmd.Flags().GetFloat64("speed")
	if speed <= 0 {
		speed = 1.0
	}

	const outputSampleRate = int32(44100)
	// Render at adjusted rate: higher rate = slower playback when written at 44100
	renderSampleRate := int32(float64(outputSampleRate) / speed)
	settings := meltysynth.NewSynthesizerSettings(renderSampleRate)
	synthesizer, err := meltysynth.NewSynthesizer(soundFont, settings)
	if err != nil {
		slog.Error("failed to create synthesizer", "error", err)
		return
	}
	synthesizer.MasterVolume = 1.0

	// Set program/bank if specified
	program, _ := cmd.Flags().GetInt("program")
	bank, _ := cmd.Flags().GetInt("bank")
	if program >= 0 {
		synthesizer.ProcessMidiMessage(0, 0xB0, 0x00, int32(bank)) // Bank select MSB
		synthesizer.ProcessMidiMessage(0, 0xC0, int32(program), 0) // Program change
	}

	var left, right []float32

	if midiFile != "" {
		// Render MIDI file directly
		left, right, err = renderMidiFile(midiFile, synthesizer)
		if err != nil {
			slog.Error("failed to render MIDI", "error", err)
			return
		}
	} else {
		// Render from score notes
		left, right = renderMidiNotes(midiNotes, synthesizer, int(renderSampleRate))
	}

	slog.Info("Rendered audio",
		"samples", len(left),
		"duration_s", fmt.Sprintf("%.1f", float64(len(left))/float64(outputSampleRate)))

	// Normalize to ~70% of full scale (matches doremi output level)
	var peak float32
	for _, s := range left {
		if s > peak {
			peak = s
		} else if -s > peak {
			peak = -s
		}
	}
	for _, s := range right {
		if s > peak {
			peak = s
		} else if -s > peak {
			peak = -s
		}
	}
	if peak > 0 {
		const targetPeak = 0.7
		gain := float32(targetPeak) / peak
		for i := range left {
			left[i] *= gain
			right[i] *= gain
		}
		slog.Info("Normalized audio", "original_peak", fmt.Sprintf("%.4f", peak), "gain", fmt.Sprintf("%.2f", gain))
	}

	// Write WAV
	writeWAV := func(w io.Writer) error {
		return writeStereoFloat32WAV(w, left, right, int(outputSampleRate))
	}

	if toStdout {
		if err := writeWAV(os.Stdout); err != nil {
			slog.Error("failed to write WAV to stdout", "error", err)
		}
	} else {
		f, err := os.OpenFile(outFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			slog.Error("failed to create output file", "error", err)
			return
		}
		defer f.Close()
		if err := writeWAV(f); err != nil {
			slog.Error("failed to write WAV", "error", err)
			return
		}
		slog.Info("Written", "path", outFile)
	}
}

func scoreToMidiNotes(scoreFile string) ([]dsp.MidiNote, error) {
	f, err := os.Open(scoreFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("empty score")
	}

	var notes []dsp.MidiNote
	currentMs := 0

	for _, rec := range records[1:] {
		if len(rec) < 2 {
			continue
		}
		noteName := strings.TrimSpace(rec[0])
		durMs, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}

		if noteName != "-" && noteName != "rest" && noteName != "R" {
			midiNote := dsp.NoteNameToMIDI(noteName)

			// Optional velocity column, otherwise auto-velocity by octave
			var velocity uint8
			if len(rec) >= 3 {
				if v, err := strconv.Atoi(strings.TrimSpace(rec[2])); err == nil && v > 0 && v <= 127 {
					velocity = uint8(v)
				}
			}
			if velocity == 0 {
				velocity = autoVelocity(midiNote)
			}

			notes = append(notes, dsp.MidiNote{
				Note:     midiNote,
				Velocity: velocity,
				StartMs:  currentMs,
				DurMs:    durMs,
				Channel:  0,
			})
		}
		currentMs += durMs
	}
	return notes, nil
}

// autoVelocity assigns velocity based on octave for more realistic dynamics.
// Bass notes are softer (fingerpicked), melody notes are louder.
func autoVelocity(midiNote uint8) uint8 {
	octave := int(midiNote) / 12
	switch {
	case octave <= 3: // C2-B3: bass, thumb-plucked
		return 75
	case octave == 4: // C4-B4: mid-range
		return 90
	default: // C5+: melody
		return 100
	}
}

func renderMidiFile(midiPath string, synth *meltysynth.Synthesizer) ([]float32, []float32, error) {
	f, err := os.Open(midiPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	mid, err := meltysynth.NewMidiFile(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse MIDI: %w", err)
	}

	sequencer := meltysynth.NewMidiFileSequencer(synth)
	sequencer.Play(mid, false)

	// Render in chunks
	sampleRate := synth.SampleRate
	totalSamples := int(mid.GetLength().Seconds()*float64(sampleRate)) + int(sampleRate) // +1s tail
	left := make([]float32, totalSamples)
	right := make([]float32, totalSamples)

	const chunkSize = 4096
	for offset := 0; offset < totalSamples; offset += chunkSize {
		end := offset + chunkSize
		if end > totalSamples {
			end = totalSamples
		}
		sequencer.Render(left[offset:end], right[offset:end])
	}

	return left, right, nil
}

func renderMidiNotes(notes []dsp.MidiNote, synth *meltysynth.Synthesizer, sampleRate int) ([]float32, []float32) {
	if len(notes) == 0 {
		return nil, nil
	}

	// Find total duration
	totalMs := 0
	for _, n := range notes {
		end := n.StartMs + n.DurMs
		if end > totalMs {
			totalMs = end
		}
	}
	totalMs += 500 // tail for release

	totalSamples := sampleRate * totalMs / 1000
	left := make([]float32, totalSamples)
	right := make([]float32, totalSamples)

	// Build event list sorted by time
	type synthEvent struct {
		samplePos int
		noteOn    bool
		channel   int32
		note      int32
		velocity  int32
	}

	var events []synthEvent
	for _, n := range notes {
		startSample := sampleRate * n.StartMs / 1000
		endSample := sampleRate * (n.StartMs + n.DurMs) / 1000
		events = append(events,
			synthEvent{samplePos: startSample, noteOn: true, channel: int32(n.Channel), note: int32(n.Note), velocity: int32(n.Velocity)},
			synthEvent{samplePos: endSample, noteOn: false, channel: int32(n.Channel), note: int32(n.Note), velocity: 0},
		)
	}

	// Sort events by sample position
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].samplePos < events[j-1].samplePos; j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}

	// Render in segments between events
	eventIdx := 0
	pos := 0
	for pos < totalSamples {
		// Process all events at current position
		for eventIdx < len(events) && events[eventIdx].samplePos <= pos {
			ev := events[eventIdx]
			if ev.noteOn {
				synth.NoteOn(ev.channel, ev.note, ev.velocity)
			} else {
				synth.NoteOff(ev.channel, ev.note)
			}
			eventIdx++
		}

		// Render until next event or end
		nextPos := totalSamples
		if eventIdx < len(events) {
			nextPos = events[eventIdx].samplePos
		}
		if nextPos > totalSamples {
			nextPos = totalSamples
		}

		chunkLen := nextPos - pos
		if chunkLen > 0 {
			synth.Render(left[pos:pos+chunkLen], right[pos:pos+chunkLen])
		}
		pos = nextPos
	}

	return left, right
}

func writeStereoFloat32WAV(w io.Writer, left, right []float32, sampleRate int) error {
	nSamples := len(left)
	// Convert float32 stereo to interleaved int16 bytes
	audio := make([]byte, nSamples*2*2) // 2 channels * 2 bytes per sample
	for i := 0; i < nSamples; i++ {
		lSample := int16(clampF32(left[i]) * math.MaxInt16)
		rSample := int16(clampF32(right[i]) * math.MaxInt16)
		offset := i * 4
		audio[offset] = byte(lSample)
		audio[offset+1] = byte(lSample >> 8)
		audio[offset+2] = byte(rSample)
		audio[offset+3] = byte(rSample >> 8)
	}

	wavWriter := wav.NewWriter(w,
		uint32(nSamples),
		2,
		uint32(sampleRate),
		16)
	_, err := wavWriter.Write(audio)
	return err
}

func clampF32(v float32) float32 {
	if v > 1.0 {
		return 1.0
	}
	if v < -1.0 {
		return -1.0
	}
	return v
}

// ScoreToMIDIBytes converts a score CSV file to MIDI file bytes in memory.
func ScoreToMIDIBytes(scoreFile string, tempo int) ([]byte, error) {
	notes, err := scoreToMidiNotes(scoreFile)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := dsp.WriteMIDIFile(&buf, notes, tempo); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
