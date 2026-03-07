/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drgolem/musiclab/dsp"
	"github.com/spf13/cobra"
	"github.com/youpy/go-wav"
)

type scoreNote struct {
	note string
	dur  time.Duration
}

var doremiCmd = &cobra.Command{
	Use:   "doremi",
	Short: "Synthesize a WAV file from a musical score CSV",
	Long: `Synthesize audio from a musical score CSV file or built-in demos.

Simple format:  note,dur (sequential notes)
Multi-track:    track,wave,note,start,dur (overlapping instruments)

Use "-" or "rest" as note name for silence.
Waveforms: sine, square, triangle, sawtooth (or saw).

Examples:
  musiclab doremi --score scores/ode_to_joy.csv --out ode.wav
  musiclab doremi --score scores/canon_harmony.csv --out canon.wav
  musiclab doremi --score scores/greensleeves.csv --stdout | musictools play -
  musiclab doremi --list
  musiclab doremi --out demo.wav`,
	Run: doDoremiCmd,
}

func init() {
	rootCmd.AddCommand(doremiCmd)

	doremiCmd.Flags().String("out", "doremi.wav", "output wav file")
	doremiCmd.Flags().String("score", "", "score data in csv format")
	doremiCmd.Flags().Bool("list", false, "list available score files in scores/ directory")
	doremiCmd.Flags().Bool("stdout", false, "write WAV to stdout (for piping)")
	doremiCmd.Flags().String("wave", "sine", "waveform for simple scores: sine, square, triangle, sawtooth, guitar, pluck")
}

const (
	defaultSampleRate    = 44100
	defaultNumChannels   = 2
	defaultBitsPerSample = 16
	defaultAmplitude     = 0.7
)

func doDoremiCmd(cmd *cobra.Command, args []string) {
	doList, _ := cmd.Flags().GetBool("list")
	if doList {
		listScores()
		return
	}

	toStdout, _ := cmd.Flags().GetBool("stdout")

	outFileName, err := cmd.Flags().GetString("out")
	if err != nil {
		slog.Error("failed to get out flag", "error", err)
		return
	}

	scoreFileName, err := cmd.Flags().GetString("score")
	if err != nil {
		slog.Error("failed to get score flag", "error", err)
		return
	}

	if !toStdout && !cmd.Flags().Changed("out") && scoreFileName != "" {
		base := filepath.Base(scoreFileName)
		outFileName = strings.TrimSuffix(base, filepath.Ext(base)) + ".wav"
	}

	var writeWAV func(w io.Writer) error

	if scoreFileName != "" {
		records, err := readCSVRecords(scoreFileName)
		if err != nil {
			slog.Error("failed to read score CSV", "error", err)
			return
		}
		if isMultiTrack(records) {
			events, err := parseMultiTrackRecords(records)
			if err != nil {
				slog.Error("failed to parse multi-track CSV", "error", err)
				return
			}
			writeWAV = func(w io.Writer) error { return writeMultiTrackToWriter(w, events) }
		} else {
			score, err := parseSimpleRecords(records)
			if err != nil {
				slog.Error("failed to parse score CSV", "error", err)
				return
			}
			waveform, _ := cmd.Flags().GetString("wave")
			writeWAV = func(w io.Writer) error { return writeScoreToWriter(w, score, waveform) }
		}
	} else {
		score := doremiDemoScore()
		writeWAV = func(w io.Writer) error { return writeScoreToWriter(w, score, "sine") }
	}

	if toStdout {
		if err := writeWAV(os.Stdout); err != nil {
			slog.Error("failed to write WAV to stdout", "error", err)
		}
	} else {
		fOut, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			slog.Error("failed to open output file", "error", err)
			return
		}
		defer fOut.Close()
		if err := writeWAV(fOut); err != nil {
			slog.Error("failed to write WAV", "error", err)
		}
	}
}

func listScores() {
	entries, err := os.ReadDir("scores")
	if err != nil {
		slog.Error("failed to read scores directory", "error", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			fmt.Printf("  scores/%s\n", e.Name())
		}
	}
}

func readCSVRecords(scoreFileName string) ([][]string, error) {
	f, err := os.Open(scoreFileName)
	if err != nil {
		return nil, fmt.Errorf("unable to read input file %s: %w", scoreFileName, err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("unable to parse CSV %s: %w", scoreFileName, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("empty score in %s", scoreFileName)
	}
	return records, nil
}

func isMultiTrack(records [][]string) bool {
	if len(records) == 0 || len(records[0]) < 5 {
		return false
	}
	h := strings.ToLower(strings.TrimSpace(records[0][0]))
	return h == "track"
}

func parseSimpleRecords(records [][]string) ([]scoreNote, error) {
	score := make([]scoreNote, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) < 2 {
			continue
		}
		note := strings.TrimSpace(rec[0])
		durVal, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			return nil, fmt.Errorf("unable to parse duration: %w", err)
		}
		score = append(score, scoreNote{note: note, dur: time.Duration(durVal) * time.Millisecond})
	}
	return score, nil
}

func parseMultiTrackRecords(records [][]string) ([]dsp.TrackEvent, error) {
	events := make([]dsp.TrackEvent, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) < 5 {
			return nil, fmt.Errorf("row %d: expected 5 columns (track,wave,note,start,dur), got %d", i+2, len(rec))
		}
		track, err := strconv.Atoi(strings.TrimSpace(rec[0]))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid track number: %w", i+2, err)
		}
		wave := strings.TrimSpace(rec[1])
		note := strings.TrimSpace(rec[2])
		startMs, err := strconv.Atoi(strings.TrimSpace(rec[3]))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid start time: %w", i+2, err)
		}
		durMs, err := strconv.Atoi(strings.TrimSpace(rec[4]))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid duration: %w", i+2, err)
		}
		events = append(events, dsp.TrackEvent{
			Track:   track,
			Wave:    wave,
			Note:    note,
			StartMs: startMs,
			DurMs:   durMs,
		})
	}
	return events, nil
}

func doremiDemoScore() []scoreNote {
	doremi := []string{
		"C4", "D4", "E4", "F4", "G4", "A4", "B4", "C5",
		"B4", "A4", "G4", "F4", "E4", "D4", "C4",
		"C5", "D5", "E5", "F5", "G5", "A5", "B5",
		"C6", "D6", "E6", "F6", "G6", "A6", "B6",
		"C7", "D7", "E7", "F7", "G7", "A7", "B7",
		"C8", "D8", "E8", "F8", "G8", "A8", "B8",
	}

	dur := 800 * time.Millisecond
	score := make([]scoreNote, 0, len(doremi))
	for _, note := range doremi {
		score = append(score, scoreNote{note: note, dur: dur})
	}
	return score
}

func isRest(note string) bool {
	return note == "-" || note == "rest" || note == "R"
}

func writeScoreToWriter(w io.Writer, score []scoreNote, waveform string) error {
	audio := make([]byte, 0)
	nSamples := 0

	useKS := waveform == "guitar" || waveform == "pluck"

	for _, sc := range score {
		if isRest(sc.note) {
			n := defaultSampleRate * int(sc.dur.Milliseconds()) / 1000
			audio = append(audio, make([]byte, n*defaultNumChannels*defaultBitsPerSample/8)...)
			nSamples += n
			continue
		}

		freq := dsp.NoteToFrequency(sc.note)

		if useKS {
			n := defaultSampleRate * int(sc.dur.Milliseconds()) / 1000
			mono := dsp.KarplusStrong(freq, defaultSampleRate, n, 0.996)
			// Convert to stereo int16 bytes
			stereo := make([]float64, n*2)
			for i, s := range mono {
				sample := defaultAmplitude * s
				stereo[i*2] = sample
				stereo[i*2+1] = sample
			}
			audio = append(audio, dsp.Float64ToInt16Bytes(stereo)...)
			nSamples += n
			continue
		}

		n1, a1 := generateTone(sc.dur, defaultSampleRate, float32(freq),
			sampleADSRAmpl(defaultAmplitude, defaultSampleRate, sc.dur))
		nSamples += n1
		audio = append(audio, a1...)
	}

	wavWriter := wav.NewWriter(w,
		uint32(nSamples),
		uint16(defaultNumChannels),
		uint32(defaultSampleRate),
		uint16(defaultBitsPerSample))

	if _, err := wavWriter.Write(audio); err != nil {
		return fmt.Errorf("writing WAV data: %w", err)
	}
	return nil
}

func writeMultiTrackToWriter(w io.Writer, events []dsp.TrackEvent) error {
	samples := dsp.MixTracks(events, defaultSampleRate)
	// samples is interleaved stereo float64
	nSamples := len(samples) / defaultNumChannels
	audio := dsp.Float64ToInt16Bytes(samples)

	wavWriter := wav.NewWriter(w,
		uint32(nSamples),
		uint16(defaultNumChannels),
		uint32(defaultSampleRate),
		uint16(defaultBitsPerSample))

	if _, err := wavWriter.Write(audio); err != nil {
		return fmt.Errorf("writing WAV data: %w", err)
	}
	return nil
}

func sampleADSRAmpl(ampl float64, sampleRate int, dur time.Duration) func(currentframe int) float64 {
	duration := dur.Seconds()
	attack := 0.1 * duration
	decay := 0.3 * duration
	release := 0.3 * duration
	sustain := 0.8 * ampl
	return func(currentframe int) float64 {
		return ADSR(ampl, duration,
			attack, decay, sustain, release,
			float64(sampleRate),
			currentframe)
	}
}

// Attack, Decay, Sustain, Release
// A,D,R - ratios in duration of total signal
// Sustain - amplitude of sustain signal
func ADSR(maxamp, duration, attacktime, decaytime, sus, releasetime, controlrate float64, currentframe int) float64 {
	dur := duration * controlrate
	at := attacktime * controlrate
	dt := decaytime * controlrate
	rt := releasetime * controlrate
	cnt := float64(currentframe)

	amp := 0.0
	if cnt < dur {
		if cnt <= at {
			// attack
			amp = cnt * (maxamp / at)
		} else if cnt <= (at + dt) {
			// decay
			amp = ((sus-maxamp)/dt)*(cnt-at) + maxamp
		} else if cnt <= dur-rt {
			// sustain
			amp = sus
		} else if cnt > (dur - rt) {
			// release
			amp = -(sus/rt)*(cnt-(dur-rt)) + sus
		}
	}

	return amp
}

func generateTone(dur time.Duration, sampleRate int, freq float32, amplFn func(int) float64) (int, []byte) {
	nSamples := sampleRate * int(dur.Milliseconds()) / 1000
	step := float64(freq) / float64(sampleRate)

	var phase float64
	// 2 channels
	// 2 byte per audio frame
	dataBuffer := make([]byte, nSamples*2*2)

	curFrame := 0
	for i := 0; i < 2*2*nSamples; {
		_, val := math.Modf(amplFn(curFrame) * math.Sin(2*math.Pi*phase))
		frame := int16(val * 0x7FFF)
		// left channel
		dataBuffer[i] = byte(frame)
		i++
		dataBuffer[i] = byte(frame >> 8)
		i++
		// right channel
		dataBuffer[i] = byte(frame)
		i++
		dataBuffer[i] = byte(frame >> 8)
		i++

		_, phase = math.Modf(phase + step)
		curFrame++
	}
	return nSamples, dataBuffer
}
