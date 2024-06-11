/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/spf13/cobra"
	"github.com/youpy/go-wav"

	soxr "github.com/zaf/resample"
)

// resampleCmd represents the resample command
var resampleCmd = &cobra.Command{
	Use:   "transform",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doResampleCmd,
}

func init() {
	rootCmd.AddCommand(resampleCmd)

	resampleCmd.Flags().String("in", "", "input file to resample")
	resampleCmd.Flags().Int("new-samplerate", 48000, "new samplerate")
	resampleCmd.Flags().String("out", "out_transformed.wav", "output wav file with a new samplerate")
	resampleCmd.Flags().Bool("mono", false, "output to mono signal")
}

func doResampleCmd(cmd *cobra.Command, args []string) {
	inFileName, err := cmd.Flags().GetString("in")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	if _, err := os.Stat(inFileName); os.IsNotExist(err) {
		fmt.Printf("path [%s] does not exist\n", inFileName)
		return
	}

	newSampleRate, err := cmd.Flags().GetInt("new-samplerate")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	outFileName, err := cmd.Flags().GetString("out")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	convertToMono, err := cmd.Flags().GetBool("mono")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	const framesPerBuffer = 2048

	audioDataChan, audioFormat, closeFn, err := audiosource.MusicAudioProducer(ctx, inFileName, audiosource.WithFramesPerBuffer(framesPerBuffer))
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer closeFn()

	fmt.Printf("Resamping: %s\n", inFileName)
	fmt.Printf("Encoding: Signed 16bit\n")
	fmt.Printf("Channels: %d\n", audioFormat.NumChannels)
	fmt.Printf("Input Sample Rate: %d\n", audioFormat.SampleRate)
	fmt.Printf("Output Sample Rate: %d\n", newSampleRate)

	inSamplesCnt := 0

	audioData := make([]byte, 0)

	for pct := range audioDataChan {
		inSamplesCnt += pct.SamplesCount

		audioData = append(audioData, pct.Audio[:pct.SamplesCount*4]...)
	}

	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)

	res, err := soxr.New(bufWriter,
		float64(audioFormat.SampleRate),
		float64(newSampleRate),
		audioFormat.NumChannels,
		soxr.I16,
		soxr.HighQ)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	ns, err := res.Write(audioData)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	fmt.Printf("%v\n", ns)
	res.Close()

	outSamplesCnt := buf.Len() / (audioFormat.NumChannels * audioFormat.BitsPerSample / 8)

	outNumChannels := audioFormat.NumChannels

	var outputData []byte

	if convertToMono && audioFormat.NumChannels == 2 {
		var bufMono bytes.Buffer
		bufMonoWriter := bufio.NewWriter(&bufMono)

		stereoData := buf.Bytes()

		idx := 0
		for idx < len(stereoData) {
			chSample := [2]int16{}
			for ch := 0; ch < 2; ch++ {
				b0 := int16(stereoData[idx])
				idx++
				b1 := int16(stereoData[idx])
				idx++

				chSample[ch] = int16((b1 << 8) | b0)
			}

			t := chSample[0]/2 + chSample[1]/2

			bufMonoWriter.WriteByte(byte(t & 0xFF))
			bufMonoWriter.WriteByte(byte((t >> 8) & 0xFF))
		}

		bufMonoWriter.Flush()

		outputData = bufMono.Bytes()
		outNumChannels = 1
	} else {
		outputData = buf.Bytes()
	}

	fOut, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer fOut.Close()

	wavWriter := wav.NewWriter(fOut,
		uint32(outSamplesCnt),
		uint16(outNumChannels),
		uint32(newSampleRate),
		uint16(audioFormat.BitsPerSample))

	wavWriter.Write(outputData)

	fmt.Printf("input samples: %d\n", inSamplesCnt)
	fmt.Printf("output samples: %d\n", outSamplesCnt)
}
