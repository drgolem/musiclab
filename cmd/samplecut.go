package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/spf13/cobra"
	"github.com/youpy/go-wav"
)

// fftCmd represents the spectrogram command
var samplecutCmd = &cobra.Command{
	Use:   "samplecut",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doSamplecutCmd,
}

func init() {
	rootCmd.AddCommand(samplecutCmd)

	samplecutCmd.Flags().String("in", "", "file to cut")
	samplecutCmd.Flags().String("out", "out_cut.wav", "output wav file")
	samplecutCmd.Flags().String("start", "10s5ms", "start")
	samplecutCmd.Flags().String("duration", "30s", "duration")
}

func doSamplecutCmd(cmd *cobra.Command, args []string) {
	inFileName, err := cmd.Flags().GetString("in")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	if _, err := os.Stat(inFileName); os.IsNotExist(err) {
		fmt.Printf("path [%s] does not exist\n", inFileName)
		return
	}
	outFileName, err := cmd.Flags().GetString("out")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	startStr, err := cmd.Flags().GetString("start")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	start, err := time.ParseDuration(startStr)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	durtStr, err := cmd.Flags().GetString("duration")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	dur, err := time.ParseDuration(durtStr)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	const framesPerBuffer = 2048

	audioStream, err := audiosource.MusicAudioProducer(ctx, inFileName, audiosource.WithFramesPerBuffer(framesPerBuffer))
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer audioStream.CancelFunc()

	fmt.Printf("Samplecut: %s\n", inFileName)
	fmt.Printf("Channels: %d\n", audioStream.AudioFormat.Channels)
	fmt.Printf("Input Sample Rate: %d\n", audioStream.AudioFormat.SampleRate)

	outSamplesCnt := int(dur.Seconds() * float64(audioStream.AudioFormat.SampleRate))
	startSamplesPos := int(start.Seconds() * float64(audioStream.AudioFormat.SampleRate))

	fmt.Printf("in %s\n", inFileName)
	fmt.Printf("out %s\n", outFileName)
	fmt.Printf("[%v:%v]\n", start, dur)

	fmt.Printf("out samples: %d\n", outSamplesCnt)

	// 1 sample - num channels * bits per sample
	frameByteSize := audioStream.AudioFormat.Channels * audioStream.AudioFormat.BitsPerSample / 8

	audioData := make([]byte, 0)
	samplesCnt := 0
	samplesPos := 0
	for pct := range audioStream.Stream {

		if startSamplesPos > samplesPos+pct.SamplesCount {
			samplesPos += pct.SamplesCount
			continue
		}

		pctStartPos := 0
		pctSamplesCount := pct.SamplesCount
		if startSamplesPos > samplesPos && startSamplesPos < samplesPos+pct.SamplesCount {
			pctStartPos = startSamplesPos - samplesPos
		}

		if samplesCnt+pct.SamplesCount > outSamplesCnt {
			pctSamplesCount = outSamplesCnt - samplesCnt
		}

		samplesPos += pct.SamplesCount
		samplesCnt += pct.SamplesCount

		pctByteStart := pctStartPos * frameByteSize
		pctByteEnd := pctSamplesCount * frameByteSize
		audioData = append(audioData, pct.Audio[pctByteStart:pctByteEnd]...)

		if samplesCnt >= outSamplesCnt {
			break
		}
	}

	fOut, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer fOut.Close()

	wavWriter := wav.NewWriter(fOut,
		uint32(outSamplesCnt),
		uint16(audioStream.AudioFormat.Channels),
		uint32(audioStream.AudioFormat.SampleRate),
		uint16(audioStream.AudioFormat.BitsPerSample))

	wavWriter.Write(audioData)
}
