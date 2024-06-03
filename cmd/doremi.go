/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/youpy/go-wav"
)

type scoreNote struct {
	note string
	dur  time.Duration
}

// doremiCmd represents the doremi command
var doremiCmd = &cobra.Command{
	Use:   "doremi",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doDoremiCmd,
}

func init() {
	rootCmd.AddCommand(doremiCmd)

	doremiCmd.Flags().String("out", "doremi.wav", "output wav file")
	doremiCmd.Flags().String("score", "", "score data in csv format")
}

func doDoremiCmd(cmd *cobra.Command, args []string) {

	outFileName, err := cmd.Flags().GetString("out")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	scoreFileName, err := cmd.Flags().GetString("score")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	const sampleRate = 44100
	const outNumChannels = 2
	const bitsPerSample = 16

	ampl := 0.7

	if scoreFileName != "" {
		if _, err := os.Stat(scoreFileName); os.IsNotExist(err) {
			fmt.Printf("path [%s] does not exist\n", scoreFileName)
			return
		}

		f, err := os.Open(scoreFileName)
		if err != nil {
			log.Fatal("Unable to read input file "+scoreFileName, err)
		}
		defer f.Close()

		csvReader := csv.NewReader(f)
		records, err := csvReader.ReadAll()
		if err != nil {
			log.Fatal("Unable to parse file as CSV for "+scoreFileName, err)
		}

		if len(records) < 1 {
			log.Fatal("Empty records in " + scoreFileName)
		}

		score := make([]scoreNote, 0)

		for _, rec := range records[1:] {
			fmt.Printf("%v\n", rec)

			note := rec[0]
			durVal, err := strconv.Atoi(rec[1])
			if err != nil {
				log.Fatal("Unable to parse file as CSV for "+scoreFileName, err)
			}
			dur := time.Duration(durVal) * time.Millisecond
			score = append(score, scoreNote{note: note, dur: dur})

			audio := make([]byte, 0)
			nSamples := 0

			for _, sc := range score {
				freq := noteToFrequency(sc.note)

				n1, a1 := generateTone(sc.dur, sampleRate, float32(freq),
					sampleADSRAmpl(ampl, sampleRate, sc.dur))
				nSamples += n1
				audio = append(audio, a1...)
			}

			fOut2, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				panic(err)
			}
			defer fOut2.Close()

			wavWriter2 := wav.NewWriter(fOut2,
				uint32(nSamples),
				uint16(outNumChannels),
				uint32(sampleRate),
				uint16(bitsPerSample))

			wavWriter2.Write(audio)
		}
	} else {

		doremi := []string{
			"C4", "D4", "E4", "F4", "G4", "A4", "B4", "C5",
			"B4", "A4", "G4", "F4", "E4", "D4", "C4",
			"C5", "D5", "E5", "F5", "G5", "A5", "B5",
			"C6", "D6", "E6", "F6", "G6", "A6", "B6",
			"C7", "D7", "E7", "F7", "G7", "A7", "B7",
			"C8", "D8", "E8", "F8", "G8", "A8", "B8",
		}

		nSamples := 0
		audio := make([]byte, 0)

		dur := 800 * time.Millisecond

		for _, note := range doremi {
			freq := noteToFrequency(note)

			n1, a1 := generateTone(dur, sampleRate, float32(freq),
				sampleADSRAmpl(ampl, sampleRate, dur))
			nSamples += n1
			audio = append(audio, a1...)
		}

		fOut, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		defer fOut.Close()

		wavWriter := wav.NewWriter(fOut,
			uint32(nSamples),
			uint16(outNumChannels),
			uint32(sampleRate),
			uint16(bitsPerSample))

		wavWriter.Write(audio)

		// BPM - 48
		// 48 beats per minute
		// 2/4 - 2 beats per measure, quarter note 1 beat
		// tempo Allegro con brio - 88 BPM
		// 88 beats - 60 sec
		//  1 beat  - 60 / 88 = 682 msec

		const quarteNoteDur = 682 * time.Millisecond

		score := []scoreNote{
			{"G4", quarteNoteDur / 2},
			{"G4", quarteNoteDur / 2},
			{"G4", quarteNoteDur / 2},
			{"Eb4", quarteNoteDur * 2},
			{"F4", quarteNoteDur / 2},
			{"F4", quarteNoteDur / 2},
			{"F4", quarteNoteDur / 2},
			{"D4", quarteNoteDur * 2},
		}

		audio = make([]byte, 0)
		nSamples = 0

		for _, sc := range score {
			freq := noteToFrequency(sc.note)

			n1, a1 := generateTone(sc.dur, sampleRate, float32(freq),
				sampleADSRAmpl(ampl, sampleRate, sc.dur))
			nSamples += n1
			audio = append(audio, a1...)
		}

		outFileName = "b5test.wav"
		fOut2, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		defer fOut2.Close()

		wavWriter2 := wav.NewWriter(fOut2,
			uint32(nSamples),
			uint16(outNumChannels),
			uint32(sampleRate),
			uint16(bitsPerSample))

		wavWriter2.Write(audio)
	}
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

// constant amplitude
func sampleConstantAmpl(ampl float64, _ int, _ time.Duration) func(currentframe int) float64 {
	return func(currentframe int) float64 {
		return ampl
	}
}

// exponential decay
func sampleExpDecayAmpl(ampl float64, sampleRate int, dur time.Duration) func(currentframe int) float64 {
	start := ampl
	end := 1e-4

	nSamples := float64(sampleRate) * dur.Seconds()
	decInc := math.Pow(end/start, 1/nSamples)

	return func(currentframe int) float64 {
		start *= decInc
		return start
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

func noteToFrequency(note string) float64 {

	// reference frequency: note A
	ref_freq := 440.0
	// reference octave: 4
	ref_octave := 4

	// parse note - last char is octave number
	// 7 full octaves on piano

	noteLen := len(note)

	if noteLen == 0 || noteLen > 3 {
		return ref_freq
	}

	octave := ref_octave

	if noteLen > 1 {
		// last char is octave number
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

	ref_note_idx := 9

	octaveDiff := octave - ref_octave

	freq := ref_freq * math.Pow(2, float64(octaveDiff)+float64(idx-ref_note_idx)/12)

	return freq
}
