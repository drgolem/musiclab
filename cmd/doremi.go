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
}

func doDoremiCmd(cmd *cobra.Command, args []string) {

	sampleRate := 44100

	Ampl := 0.4
	// note A
	freqA := float32(440.0)
	// note C#
	freqCsharp := float32(277.18)
	// note E
	freqE := float32(329.63)

	genAudio, err := cmd.Flags().GetBool("generate")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	if genAudio {
		// generate tone with a given duration
		//dur := 1 * time.Second
		dur := 800 * time.Millisecond

		Ampl = 0.7
		amplFn := func(dur time.Duration) func(currentframe int) float64 {
			return func(currentframe int) float64 {
				//return Ampl
				attack := 0.1 * dur.Seconds()
				decay := 0.3 * dur.Seconds()
				release := 0.3 * dur.Seconds()
				sustain := 0.8 * Ampl
				return ADSR(Ampl, dur.Seconds(),
					attack, decay, sustain, release,
					float64(sampleRate),
					currentframe)
			}
		}

		nSamples, audio := generateTone(dur, sampleRate, freqA, amplFn(dur))

		n1, a1 := generateTone(dur, sampleRate, freqCsharp, amplFn(dur))
		nSamples += n1
		audio = append(audio, a1...)

		n1, a1 = generateTone(dur, sampleRate, freqE, amplFn(dur))
		nSamples += n1
		audio = append(audio, a1...)

		outFileName := "test.wav"
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
		fmt.Printf("Format supported: %v, sampleRate: %f\n", outStreamParams, sampleRate)
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
