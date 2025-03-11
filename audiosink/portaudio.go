package audiosink

import (
	"context"
	"fmt"

	"github.com/drgolem/go-portaudio/portaudio"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/types"
)

type AudioSink interface {
	Play(ctx context.Context) error
	Close(ctx context.Context) error
}

type portAudioSink struct {
	audioFormat     types.FrameFormat
	deviceIdx       int
	framesPerBuffer int
	stream          *portaudio.PaStream
	audioPctChan    <-chan audiosource.AudioSamplesPacket
}

func NewPortAudioSink(deviceIdx int,
	framesPerBuffer int,
	audioPctChan <-chan audiosource.AudioSamplesPacket,
) (AudioSink, error) {

	audioFormat := types.FrameFormat{
		SampleRate: 44100,
		Channels:   2,
		//BitsPerSample: 16,
	}

	sampleformat := portaudio.SampleFmtInt16

	outStreamParams := portaudio.PaStreamParameters{
		DeviceIndex:  deviceIdx,
		ChannelCount: audioFormat.Channels,
		SampleFormat: sampleformat,
	}
	stream, err := portaudio.NewStream(outStreamParams, float32(audioFormat.SampleRate))
	if err != nil {
		return nil, err
	}

	err = stream.Open(framesPerBuffer)
	if err != nil {
		return nil, err
	}

	err = stream.StartStream()
	if err != nil {
		return nil, err
	}

	ps := portAudioSink{
		audioFormat:     audioFormat,
		deviceIdx:       deviceIdx,
		framesPerBuffer: framesPerBuffer,
		stream:          stream,
		audioPctChan:    audioPctChan,
	}

	return &ps, nil
}

func (ps *portAudioSink) Play(ctx context.Context) error {
	for {
		select {
		case pkt, ok := <-ps.audioPctChan:
			if !ok {
				// channel closed, no data
				fmt.Println("PortAudioSink channel closed")
				return nil
			}

			if ps.audioFormat != pkt.Format {
				ps.audioFormat = pkt.Format

				sampleformat := portaudio.SampleFmtInt16

				outStreamParams := portaudio.PaStreamParameters{
					DeviceIndex:  ps.deviceIdx,
					ChannelCount: ps.audioFormat.Channels,
					SampleFormat: sampleformat,
				}
				var err error
				ps.stream, err = portaudio.NewStream(outStreamParams, float32(ps.audioFormat.SampleRate))
				if err != nil {
					fmt.Printf("PulseAudio: ERR: %v\n", err)
					return nil
				}

				err = ps.stream.Open(ps.framesPerBuffer)
				if err != nil {
					fmt.Printf("PulseAudio: ERR: %v\n", err)
					return nil
				}

				err = ps.stream.StartStream()
				if err != nil {
					fmt.Printf("PulseAudio: ERR: %v\n", err)
					return nil
				}
			}

			err := ps.stream.Write(pkt.SamplesCount, pkt.Audio)
			if err != nil {
				// check if context was cancelled
				if ctx.Err() != nil {
					fmt.Printf("context err: %v\n", ctx.Err())
					return nil
				}
				if err.Error() == "Output underflowed" {
					fmt.Printf("PulseAudio: Output underflowed, CONTINUE\n")
					//return err
					continue
				}
				return err
			}
		case <-ctx.Done():
			fmt.Println("PortAudioSink done")
			return nil
		}
	}
}

func (ps *portAudioSink) Close(ctx context.Context) error {

	fmt.Println("PortAudio - close")

	ps.stream.StopStream()
	ps.stream.Close()

	return nil
}
