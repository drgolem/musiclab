/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/drgolem/go-portaudio/portaudio"
	"github.com/spf13/cobra"
	"github.com/youpy/go-wav"
)

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

	doremiCmd.Flags().Bool("generate", true, "generate wav")
	doremiCmd.Flags().String("out", "doremi.wav", "output wav file")
}

func doDoremiCmd(cmd *cobra.Command, args []string) {

	sampleRate := 44100

	Ampl := 0.4
	// note A
	//freqA := float32(440.0)
	// note C#
	//freqCsharp := float32(277.18)
	// note E
	freqE := float32(329.63)

	genAudio, err := cmd.Flags().GetBool("generate")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	outFileName, err := cmd.Flags().GetString("out")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	if genAudio {
		// generate tone with a given duration
		//dur := 3 * time.Second
		dur := 800 * time.Millisecond
		//dur := 8 * time.Millisecond

		Ampl = 0.7

		amplFn := func(dur time.Duration) func(currentframe int) float64 {
			duration := dur.Seconds()
			attack := 0.1 * duration
			decay := 0.3 * duration
			release := 0.3 * duration
			sustain := 0.8 * Ampl
			return func(currentframe int) float64 {
				return ADSR(Ampl, duration,
					attack, decay, sustain, release,
					float64(sampleRate),
					currentframe)
			}
		}

		/*
			// constant amplitude
			amplFn := func(dur time.Duration) func(currentframe int) float64 {
				return func(currentframe int) float64 {
					return Ampl
				}
			}
		*/

		/*
			// exponential decay
			amplFn := func(dur time.Duration) func(currentframe int) float64 {
				start := Ampl
				end := 1e-4

				nSamples := float64(sampleRate) * dur.Seconds()
				decInc := math.Pow(end/start, 1/nSamples)

				return func(currentframe int) float64 {
					start *= decInc
					return start
				}
			}
		*/

		/*
			nSamples, audio := generateTone(dur, sampleRate, freqA, amplFn(dur))

			n1, a1 := generateTone(dur, sampleRate, freqCsharp, amplFn(dur))
			nSamples += n1
			audio = append(audio, a1...)

			n1, a1 = generateTone(dur, sampleRate, freqE, amplFn(dur))
			nSamples += n1
			audio = append(audio, a1...)
		*/

		/*
			doremi := []string{
				"C4", "D4", "E4", "F4", "G4", "A4", "B4", "C5",
				"B4", "A4", "G4", "F4", "E4", "D4", "C4",
				"C4", "C5", "C6", "C7", "C8",
				"D4", "D5", "D6", "D7", "D8",
				"E4", "E5", "E6", "E7", "E8",
				"F4", "F5", "F6", "F7", "F8",
				"G4", "G5", "G6", "G7", "G8",
				"A4", "A5", "A6", "A7", "A8",
			}
		*/
		doremi := []string{
			"C4", "D4", "E4", "F4", "G4", "A4", "B4", "C5",
			"B4", "A4", "G4", "F4", "E4", "D4", "C4",
			"C5", "D5", "E5", "F5", "G5", "A5", "B5",
			"C6", "D6", "E6", "F6", "G6", "A6", "B6",
			"C7", "D7", "E7", "F7", "G7", "A7", "B7",
			"C8", "D8", "E8", "F8", "G8", "A8", "B8",
		}

		//doremi = []string{"G4", "E4", "C4", "F4"}

		nSamples := 0
		audio := make([]byte, 0)

		for _, note := range doremi {
			freq := noteToFrequency(note)

			n1, a1 := generateTone(dur, sampleRate, float32(freq), amplFn(dur))
			nSamples += n1
			audio = append(audio, a1...)
		}

		fOut, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		defer fOut.Close()

		outNumChannels := 2
		bitsPerSample := 16

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

		score := []struct {
			note string
			dur  time.Duration
		}{
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

			n1, a1 := generateTone(sc.dur, sampleRate, float32(freq), amplFn(sc.dur))
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

		return
	}

	err = portaudio.Initialize()
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer portaudio.Terminate()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	devCnt, err := portaudio.GetDeviceCount()
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
	} else {
		fmt.Printf("device count: %d\n", devCnt)
	}

	for devIdx := 0; devIdx < devCnt; devIdx++ {
		di, err := portaudio.GetDeviceInfo(devIdx)
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
		} else {
			fmt.Printf("[%d] device: %#v\n", devIdx, di)
		}
	}

	hostApiCnt, err := portaudio.GetHostApiCount()
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	fmt.Printf("Host API Info (count: %d)\n", hostApiCnt)
	for idx := 0; idx < hostApiCnt; idx++ {
		hi, err := portaudio.GetHostApiInfo(idx)
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
		} else {
			fmt.Printf("[%d] api info: %#v\n", idx, hi)
		}
	}

	outStreamParams := portaudio.PaStreamParameters{
		DeviceIndex:  1,
		ChannelCount: 2,
		SampleFormat: portaudio.SampleFmtFloat32,
		//SampleFormat: portaudio.SampleFmtInt24,
	}

	err = portaudio.IsFormatSupported(nil, &outStreamParams, float32(sampleRate))
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	} else {
		fmt.Printf("Format supported: %v, sampleRate: %d\n", outStreamParams, sampleRate)
	}

	st, err := portaudio.NewStream(outStreamParams, float32(sampleRate))
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	const framesPerBuffer = 2048

	err = st.Open(framesPerBuffer)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer st.Close()

	err = st.StartStream()
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	notesGenFn_E := noteSamplesGen(framesPerBuffer, sampleRate, Ampl, freqE)

	playFn := func(ctx context.Context) error {
		for {
			dataBuffer := notesGenFn_E()
			buf := new(bytes.Buffer)
			for _, d := range dataBuffer {
				err := binary.Write(buf, binary.LittleEndian, d)
				if err != nil {
					return err
				}
			}

			err = st.Write(framesPerBuffer, buf.Bytes())
			if err != nil {
				// check if context was cancelled
				if ctx.Err() != nil {
					fmt.Printf("context err: %v\n", ctx.Err())
					return nil
				}
				return err
			}

			select {
			case <-ctx.Done():
				return nil
			default:
			}
		}
	}

	go func() {
		err := playFn(ctx)

		// notify cancel ctx
		cancelFn()

		if err != nil {
			fmt.Printf("ERR playFn: %v\n", err)
		}
	}()

	<-ctx.Done()
	fmt.Printf("done\n")
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

func noteSamplesGen(framesPerBuffer int,
	sampleRate int, ampl float64, freq float32) func() []float32 {
	step := float64(freq) / float64(sampleRate)

	var phase float64
	dataBuffer := make([]float32, 2*framesPerBuffer)
	return func() []float32 {
		for i := 0; i < 2*framesPerBuffer; {
			val := float32(ampl * math.Sin(2*math.Pi*phase))
			dataBuffer[i] = val
			i++
			dataBuffer[i] = val
			i++
			_, phase = math.Modf(phase + step)
		}
		return dataBuffer
	}
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
