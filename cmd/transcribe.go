package cmd

import (
	"encoding/csv"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/dsp"
	"github.com/spf13/cobra"
)

var transcribeCmd = &cobra.Command{
	Use:   "transcribe",
	Short: "Transcribe a monophonic WAV file to a score CSV",
	Long: `Detect pitches in a single-instrument audio file and output a note,dur CSV.

Uses STFT with peak frequency detection per frame, then groups consecutive
frames with the same note into single entries. Works best with clean,
monophonic (single note at a time) audio.

Supported formats: WAV, FLAC, MP3, OGG Vorbis, Opus.

Examples:
  musiclab transcribe --file greensleeves.wav --out greensleeves_score.csv
  musiclab transcribe --file melody.mp3 --stdout
  musiclab transcribe --file recording.flac --start 1m30s --duration 10s --out score.csv`,
	Run: doTranscribeCmd,
}

func init() {
	rootCmd.AddCommand(transcribeCmd)

	transcribeCmd.Flags().String("file", "", "input audio file (WAV, FLAC, MP3, OGG, Opus)")
	transcribeCmd.Flags().String("out", "", "output CSV file (default: stdout)")
	transcribeCmd.Flags().Bool("stdout", false, "write CSV to stdout")
	transcribeCmd.Flags().String("start", "0s", "start position (e.g. 1m30s, 10s, 500ms)")
	transcribeCmd.Flags().String("duration", "", "duration to transcribe (e.g. 30s, 1m); empty = entire file")
	transcribeCmd.Flags().Float64("silence", 0.02, "amplitude threshold below which is silence")
	transcribeCmd.Flags().Int("frame", 4096, "FFT frame size in samples")
	transcribeCmd.Flags().Int("hop", 2048, "hop size between frames in samples")
	transcribeCmd.Flags().String("voice", "melody", "pitch detection mode: melody (top voice), bass (fundamental via HPS)")
}

type detectedNote struct {
	note     string
	startMs  int
	durMs    int
	avgFreq  float64
	maxAmpl  float64
}

func doTranscribeCmd(cmd *cobra.Command, args []string) {
	inFileName, _ := cmd.Flags().GetString("file")
	if inFileName == "" {
		slog.Error("--file is required")
		return
	}

	audioData, err := audiosource.AudioSamplesFromFile(inFileName)
	if err != nil {
		slog.Error("failed to read audio", "error", err)
		return
	}

	// Trim to start/duration range
	startStr, _ := cmd.Flags().GetString("start")
	startDur, err := time.ParseDuration(startStr)
	if err != nil {
		slog.Error("invalid start value", "value", startStr, "error", err)
		return
	}
	startSample := int(startDur.Seconds() * float64(audioData.SampleRate))
	if startSample > len(audioData.Audio) {
		slog.Error("start position beyond end of file",
			"start", startStr,
			"file_duration", time.Duration(float64(len(audioData.Audio))/float64(audioData.SampleRate)*float64(time.Second)))
		return
	}
	audioData.Audio = audioData.Audio[startSample:]

	durStr, _ := cmd.Flags().GetString("duration")
	if durStr != "" {
		dur, err := time.ParseDuration(durStr)
		if err != nil {
			slog.Error("invalid duration value", "value", durStr, "error", err)
			return
		}
		durSamples := int(dur.Seconds() * float64(audioData.SampleRate))
		if durSamples < len(audioData.Audio) {
			audioData.Audio = audioData.Audio[:durSamples]
		}
	}

	silenceThresh, _ := cmd.Flags().GetFloat64("silence")
	frameSize, _ := cmd.Flags().GetInt("frame")
	hopSize, _ := cmd.Flags().GetInt("hop")

	slog.Info("Transcribing",
		"file", inFileName,
		"sample_rate", audioData.SampleRate,
		"samples", len(audioData.Audio),
		"start", startStr,
		"duration", durStr,
		"frame", frameSize,
		"hop", hopSize)

	voiceMode, _ := cmd.Flags().GetString("voice")

	notes := transcribeAudio(audioData.Audio, audioData.SampleRate, frameSize, hopSize, silenceThresh, voiceMode)

	slog.Info("Detected notes", "count", len(notes))

	// Output
	toStdout, _ := cmd.Flags().GetBool("stdout")
	outFileName, _ := cmd.Flags().GetString("out")

	if !toStdout && outFileName == "" {
		toStdout = true
	}

	var w *csv.Writer
	if toStdout {
		w = csv.NewWriter(os.Stdout)
	} else {
		f, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			slog.Error("failed to open output file", "error", err)
			return
		}
		defer f.Close()
		w = csv.NewWriter(f)
	}

	w.Write([]string{"note", "dur"})
	for _, n := range notes {
		w.Write([]string{n.note, strconv.Itoa(n.durMs)})
	}
	w.Flush()
}

func transcribeAudio(samples []float64, sampleRate, frameSize, hopSize int, silenceThresh float64, voiceMode ...string) []detectedNote {
	mode := "melody"
	if len(voiceMode) > 0 && voiceMode[0] != "" {
		mode = voiceMode[0]
	}

	if len(samples) < frameSize {
		return nil
	}

	st := dsp.New(hopSize, frameSize)
	spectrogram := st.STFT(samples)

	frameDurMs := float64(hopSize) / float64(sampleRate) * 1000.0

	type frameInfo struct {
		note    string
		freq    float64
		ampl    float64
		timeMs  float64
	}

	frames := make([]frameInfo, len(spectrogram))

	// Frequency range depends on voice mode
	var minFreq, maxFreq float64
	if mode == "bass" {
		minFreq, maxFreq = 30.0, 500.0
	} else {
		minFreq, maxFreq = 80.0, 2000.0
	}
	minBin := int(math.Ceil(minFreq * float64(frameSize) / float64(sampleRate)))
	maxBin := int(math.Floor(maxFreq * float64(frameSize) / float64(sampleRate)))

	for i, spec := range spectrogram {
		amp, _ := dsp.SplitSpectrum(spec)

		if maxBin >= len(amp) {
			maxBin = len(amp) - 1
		}

		// Compute total energy for silence detection
		var totalEnergy float64
		for j := minBin; j <= maxBin; j++ {
			totalEnergy += amp[j] * amp[j]
		}
		totalEnergy = math.Sqrt(totalEnergy / float64(maxBin-minBin+1))

		var peakIdx int
		var freq float64

		if mode == "bass" {
			// HPS for fundamental frequency detection
			hpsOrder := 5
			hpsLen := maxBin / hpsOrder
			if hpsLen <= minBin {
				hpsLen = maxBin
			}
			hps := make([]float64, hpsLen)
			for j := minBin; j < hpsLen; j++ {
				hps[j] = amp[j]
			}
			for h := 2; h <= hpsOrder; h++ {
				for j := minBin; j < hpsLen; j++ {
					idx := j * h
					if idx < len(amp) {
						hps[j] *= amp[idx]
					} else {
						hps[j] = 0
					}
				}
			}
			peakIdx = minBin
			peakVal := 0.0
			for j := minBin; j < hpsLen; j++ {
				if hps[j] > peakVal {
					peakVal = hps[j]
					peakIdx = j
				}
			}
		} else {
			// Melody mode: find the strongest peak in the upper frequency range
			peakIdx = minBin
			peakAmpl := 0.0
			for j := minBin; j <= maxBin; j++ {
				if amp[j] > peakAmpl {
					peakAmpl = amp[j]
					peakIdx = j
				}
			}
		}

		freq = float64(peakIdx) * float64(sampleRate) / float64(frameSize)

		note := ""
		if totalEnergy < silenceThresh {
			note = "-"
		} else {
			// Parabolic interpolation for better frequency estimate
			if peakIdx > 0 && peakIdx < len(amp)-1 {
				alpha := amp[peakIdx-1]
				beta := amp[peakIdx]
				gamma := amp[peakIdx+1]
				denom := alpha - 2*beta + gamma
				if math.Abs(denom) > 1e-10 {
					p := 0.5 * (alpha - gamma) / denom
					freq = (float64(peakIdx) + p) * float64(sampleRate) / float64(frameSize)
				}
			}
			note = dsp.FreqToNoteOctave(freq)
		}

		frames[i] = frameInfo{
			note:   note,
			freq:   freq,
			ampl:   totalEnergy,
			timeMs: float64(i) * frameDurMs,
		}
	}

	// Onset detection via spectral flux
	// Compute energy per frame, then detect sudden increases
	onsets := make([]bool, len(frames))
	if len(frames) > 1 {
		energies := make([]float64, len(frames))
		for i := range frames {
			energies[i] = frames[i].ampl
		}

		// Compute mean energy for adaptive threshold
		var meanEnergy float64
		for _, e := range energies {
			meanEnergy += e
		}
		meanEnergy /= float64(len(energies))

		// Onset detection: look for energy dip followed by rise (V-shape)
		// This catches repeated same-pitch notes with ADSR envelope
		onsetThresh := meanEnergy * 0.3
		if onsetThresh < silenceThresh {
			onsetThresh = silenceThresh
		}

		for i := 2; i < len(energies); i++ {
			// Classic spectral flux: sudden energy increase
			flux := energies[i] - energies[i-1]
			if flux > meanEnergy*1.5 {
				onsets[i] = true
				continue
			}

			// V-shape detector: energy dipped then rose (release → attack)
			if energies[i-1] < energies[i-2] && energies[i] > energies[i-1] {
				dip := energies[i-2] - energies[i-1]
				rise := energies[i] - energies[i-1]
				if dip > onsetThresh && rise > onsetThresh {
					onsets[i] = true
				}
			}
		}
	}

	// Group consecutive frames with the same note, splitting at onsets
	var notes []detectedNote
	if len(frames) == 0 {
		return notes
	}

	current := detectedNote{
		note:    frames[0].note,
		startMs: int(frames[0].timeMs),
		avgFreq: frames[0].freq,
		maxAmpl: frames[0].ampl,
	}
	count := 1

	for i := 1; i < len(frames); i++ {
		noteChanged := frames[i].note != current.note
		onsetHit := onsets[i] && frames[i].note != "-" && current.note != "-"

		if noteChanged || onsetHit {
			endMs := int(frames[i].timeMs)
			current.durMs = endMs - current.startMs
			current.avgFreq /= float64(count)
			if current.durMs > 0 {
				notes = append(notes, current)
			}
			current = detectedNote{
				note:    frames[i].note,
				startMs: endMs,
				avgFreq: frames[i].freq,
				maxAmpl: frames[i].ampl,
			}
			count = 1
		} else {
			current.avgFreq += frames[i].freq
			if frames[i].ampl > current.maxAmpl {
				current.maxAmpl = frames[i].ampl
			}
			count++
		}
	}
	// Last note
	endMs := int(float64(len(frames)) * frameDurMs)
	current.durMs = endMs - current.startMs
	current.avgFreq /= float64(count)
	if current.durMs > 0 {
		notes = append(notes, current)
	}

	// Post-processing: merge very short notes (< 80ms) into neighbors
	// Short artifacts get absorbed by the preceding note
	var merged []detectedNote
	for _, n := range notes {
		if n.durMs < 80 && len(merged) > 0 {
			// Absorb into previous note (extend its duration)
			merged[len(merged)-1].durMs += n.durMs
			continue
		}
		merged = append(merged, n)
	}

	// Filter out remaining short notes
	var filtered []detectedNote
	for _, n := range merged {
		if n.durMs >= 80 {
			filtered = append(filtered, n)
		}
	}

	return filtered
}
