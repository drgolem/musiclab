/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/drgolem/go-portaudio/portaudio"
)

// sinewaveCmd represents the sinewave command
var sinewaveCmd = &cobra.Command{
	Use:   "sinewave",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doPlaySineCmd,
}

func init() {
	rootCmd.AddCommand(sinewaveCmd)
}

func doPlaySineCmd(cmd *cobra.Command, args []string) {
	fmt.Printf("version text: %s\n", portaudio.GetVersionText())

	err := portaudio.Initialize()
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer portaudio.Terminate()

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
	sampleRate := float32(44100)
	err = portaudio.IsFormatSupported(nil, &outStreamParams, sampleRate)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	} else {
		fmt.Printf("Format supported: %v, sampleRate: %f\n", outStreamParams, sampleRate)
	}

	framesPerBuffer := 4096

	st, err := portaudio.NewStream(outStreamParams, sampleRate)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	err = st.Open(framesPerBuffer)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer st.Close()

	fmt.Println("Playing.  Press Ctrl-C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	err = st.StartStream()
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	dataBuffer := make([]float32, 2*framesPerBuffer)

	//var phaseL float64
	//var phaseR float64

	// note A
	freqA := float32(440.0)
	// note Ab
	//freqAb := float32(415.3)
	// note C#
	freqCsharp := float32(277.18)
	// note E
	freqE := float32(329.63)

	//stepL := float64(freq / sampleRate)
	//stepR := float64(freq / sampleRate)

	dataBuffer_A := make([]float32, 2*framesPerBuffer)
	dataBuffer_Csharp := make([]float32, 2*framesPerBuffer)
	dataBuffer_E := make([]float32, 2*framesPerBuffer)

	Ampl := 0.4

	//var phase float64
	var phase_A float64
	var phase_Csharp float64
	var phase_E float64
	for {
		/*
			for i := 0; i < 2*framesPerBuffer; {

				dataBuffer[i] = float32(Ampl * math.Sin(2*math.Pi*phaseL))
				i++
				_, phaseL = math.Modf(phaseL + stepL)

				dataBuffer[i] = float32(Ampl * math.Sin(2*math.Pi*phaseR))
				i++
				_, phaseR = math.Modf(phaseR + stepR)
			}
		*/
		//phase = getSamplesForNote(dataBuffer, framesPerBuffer, sampleRate, Ampl, freqA, phase)

		phase_A = getSamplesForNote(dataBuffer_A, framesPerBuffer, sampleRate, Ampl, freqA, phase_A)
		phase_Csharp = getSamplesForNote(dataBuffer_Csharp, framesPerBuffer, sampleRate, Ampl, freqCsharp, phase_Csharp)
		phase_E = getSamplesForNote(dataBuffer_E, framesPerBuffer, sampleRate, Ampl, freqE, phase_E)

		for i := 0; i < 2*framesPerBuffer; i++ {
			v := dataBuffer_A[i] + dataBuffer_Csharp[i] + dataBuffer_E[i]
			dataBuffer[i] = v
		}

		buf := new(bytes.Buffer)
		for _, d := range dataBuffer {
			err := binary.Write(buf, binary.LittleEndian, d)
			if err != nil {
				fmt.Println("binary.Write failed:", err)
				panic(err)
			}
		}

		err = st.Write(framesPerBuffer, buf.Bytes())
		if err != nil {
			panic(err)
		}

		select {
		case <-sig:
			st.StopStream()
			return
		default:
		}
	}
}

func getSamplesForNote(dataBuffer []float32, framesPerBuffer int,
	sampleRate float32, ampl float64, freq float32, phase float64) float64 {
	step := float64(freq / sampleRate)
	for i := 0; i < 2*framesPerBuffer; {
		val := float32(ampl * math.Sin(2*math.Pi*phase))
		dataBuffer[i] = val
		i++
		dataBuffer[i] = val
		i++
		_, phase = math.Modf(phase + step)
	}
	return phase
}
