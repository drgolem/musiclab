package audiosource

import (
	"bufio"
	"bytes"
	"context"
	"os/signal"
	"syscall"
)

type AudioSamples struct {
	Audio      []float64
	SampleRate int
}

func AudioSamplesFromFile(ctx context.Context, fileName string) (AudioSamples, error) {
	var out AudioSamples

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	const framesPerBuffer = 2048

	audioStream, err := MusicAudioProducer(ctx, fileName, WithFramesPerBuffer(framesPerBuffer))
	if err != nil {
		return out, err
	}
	defer audioStream.Close()

	audioFormat := audioStream.GetFormat()

	sampleRate := audioFormat.SampleRate

	inSamplesCnt := 0

	audioData := make([]byte, 0)

	for pct := range audioStream.Stream() {
		inSamplesCnt += pct.SamplesCount

		audioData = append(audioData, pct.Audio[:pct.SamplesCount*4]...)
	}

	// mix stereo to mono
	if audioFormat.Channels == 2 {
		var bufMono bytes.Buffer
		bufMonoWriter := bufio.NewWriter(&bufMono)

		stereoData := audioData
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
		audioData = bufMono.Bytes()
	}

	// convert samles to float
	audioSamples := make([]float64, 0)

	idx := 0
	for idx < len(audioData) {
		b0 := int16(audioData[idx])
		idx++
		b1 := int16(audioData[idx])
		idx++
		frameInt := int16((b1 << 8) | b0)
		frame := float64(frameInt) / 0x7FFF

		audioSamples = append(audioSamples, frame)
	}

	out.Audio = audioSamples
	out.SampleRate = sampleRate

	return out, nil
}
