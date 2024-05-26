package audioconsumer

import (
	"context"
	"fmt"

	"github.com/drgolem/go-portaudio/portaudio"

	"github.com/drgolem/musiclab/audiosource"
)

func PortaudioConsumer(deviceIdx int,
	framesPerBuffer int,
	audioFormat audiosource.AudioFormat,
	audioPctChan <-chan audiosource.AudioSamplesPacket) (playFn func(ctx context.Context) error, closeFn func() error, errRes error) {

	closeFn = func() error {
		return nil
	}
	playFn = func(ctx context.Context) error {
		return nil
	}

	sampleformat := portaudio.SampleFmtInt16

	outStreamParams := portaudio.PaStreamParameters{
		DeviceIndex:  deviceIdx,
		ChannelCount: audioFormat.NumChannels,
		SampleFormat: sampleformat,
	}
	stream, err := portaudio.NewStream(outStreamParams, float32(audioFormat.SampleRate))
	if err != nil {
		errRes = err
		return
	}

	err = stream.Open(framesPerBuffer)
	if err != nil {
		errRes = err
		return
	}

	err = stream.StartStream()
	if err != nil {
		errRes = err
		return
	}
	closeFn = func() error {
		stream.StopStream()
		stream.Close()
		return nil
	}

	playFn = func(ctx context.Context) error {
		for {
			select {
			case pkt, ok := <-audioPctChan:
				if !ok {
					// channel closed, no data
					return nil
				}
				err = stream.Write(pkt.SamplesCount, pkt.Audio)
				if err != nil {
					// check if context was cancelled
					if ctx.Err() != nil {
						fmt.Printf("context err: %v\n", ctx.Err())
						return nil
					}
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	}

	return playFn, closeFn, nil
}
