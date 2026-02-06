package cmd

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/drgolem/go-opus/opus"
	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/types"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
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

	audioStream, err := audiosource.NewMusicAudioProducer(ctx, inFileName, audiosource.WithFramesPerBuffer(framesPerBuffer))
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer audioStream.Close()

	audioFormat := audioStream.GetFormat()

	fmt.Printf("Samplecut: %s\n", inFileName)
	fmt.Printf("Channels: %d\n", audioFormat.Channels)
	fmt.Printf("Input Sample Rate: %d\n", audioFormat.SampleRate)

	outSamplesCnt := int(dur.Seconds() * float64(audioFormat.SampleRate))
	startSamplesPos := int(start.Seconds() * float64(audioFormat.SampleRate))

	fmt.Printf("in %s\n", inFileName)
	fmt.Printf("out %s\n", outFileName)
	fmt.Printf("[%v:%v]\n", start, dur)

	fmt.Printf("out samples: %d\n", outSamplesCnt)

	// 1 sample - num channels * bits per sample
	frameByteSize := audioFormat.Channels * audioFormat.BitsPerSample / 8

	var audioData []byte
	samplesCnt := 0
	samplesPos := 0
	for pct := range audioStream.Stream() {

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

	ext := filepath.Ext(outFileName)
	fileFormat := types.FileFormatType(ext)

	switch fileFormat {
	case types.FileFormat_OGG:
		//fOut, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
		//if err != nil {
		//	panic(err)
		//}
		//defer fOut.Close()

		const channels = 2
		opusEnc, err := opus.NewOpusPacketEncoder(channels)
		if err != nil {
			fmt.Printf("error create ogg opus encoder: %v", err)
			return
		}

		OpusSampleRate := 48000

		//oggWriter, err := oggwriter.NewWith(fOut, uint32(OpusSampleRate), channels)
		oggWriter, err := oggwriter.New(outFileName, uint32(OpusSampleRate), channels)
		if err != nil {
			fmt.Printf("error create ogg writer: %v", err)
			return
		}
		defer oggWriter.Close()

		pktMaker := rtp.NewPacketizer(
			1200,
			uint8(0),
			uint32(rand.Int31()),
			&codecs.OpusPayloader{},
			rtp.NewRandomSequencer(),
			uint32(OpusSampleRate),
		)

		br := bytes.NewReader(audioData)

		for {
			samples := 960

			inSamplesByteSz := samples * 2 * 2

			if br.Len() < inSamplesByteSz {
				break
			}

			audio := make([]byte, inSamplesByteSz)
			_, err := br.Read(audio)
			if err != nil {
				fmt.Printf("error read data: %v", err)
				return
			}

			packetSz := samples * 2 * 2
			packet := make([]byte, packetSz)
			outBytesSz, err := opusEnc.EncodeSamples(packet, samples, audio)
			if err != nil {
				fmt.Printf("error encode samples: %v", err)
				return
			}
			packet = packet[:outBytesSz]

			pkts := pktMaker.Packetize(packet, uint32(samples))
			for _, pkt := range pkts {
				err = oggWriter.WriteRTP(pkt)
				if err != nil {
					fmt.Printf("error write to ogg maker: %v", err)
					return
				}
			}
		}

	case types.FileFormat_WAV:
		fOut, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		defer fOut.Close()

		wavWriter := wav.NewWriter(fOut,
			uint32(outSamplesCnt),
			uint16(audioFormat.Channels),
			uint32(audioFormat.SampleRate),
			uint16(audioFormat.BitsPerSample))

		wavWriter.Write(audioData)
	default:
		fmt.Printf("unknown output format: %s\n", outFileName)
	}

}
