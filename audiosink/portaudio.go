package audiosink

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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

	slog.Info("creating audio sink",
		"device", deviceIdx,
		"frames_per_buffer", framesPerBuffer)

	ps := portAudioSink{
		deviceIdx:       deviceIdx,
		framesPerBuffer: framesPerBuffer,
		stream:          nil, // Will be initialized in Play()
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
				slog.Info("audio channel closed")
				return nil
			}

			// Initialize stream on first packet
			if ps.stream == nil {
				ps.audioFormat = pkt.Format
				if err := ps.initializeStream(); err != nil {
					return err
				}
			}

			// Check if format changed
			if ps.audioFormat != pkt.Format {
				slog.Info("audio format changed, recreating stream",
					"old_sample_rate", ps.audioFormat.SampleRate,
					"new_sample_rate", pkt.Format.SampleRate,
					"old_channels", ps.audioFormat.Channels,
					"new_channels", pkt.Format.Channels,
					"old_bits", ps.audioFormat.BitsPerSample,
					"new_bits", pkt.Format.BitsPerSample)

				if err := ps.recreateStream(pkt.Format); err != nil {
					return err
				}
			}

			if err := ps.writePacket(ctx, pkt); err != nil {
				return err
			}

		case <-ctx.Done():
			slog.Info("playback stopped")
			return nil
		}
	}
}

func (ps *portAudioSink) initializeStream() error {
	slog.Info("initializing PortAudio stream",
		"sample_rate", ps.audioFormat.SampleRate,
		"channels", ps.audioFormat.Channels,
		"bits_per_sample", ps.audioFormat.BitsPerSample)

	// Determine sample format based on bit depth
	var sampleformat portaudio.PaSampleFormat
	switch ps.audioFormat.BitsPerSample {
	case 8:
		sampleformat = portaudio.SampleFmtInt8
	case 16:
		sampleformat = portaudio.SampleFmtInt16
	default:
		return fmt.Errorf("unsupported bit depth: %d", ps.audioFormat.BitsPerSample)
	}

	outStreamParams := portaudio.PaStreamParameters{
		DeviceIndex:  ps.deviceIdx,
		ChannelCount: ps.audioFormat.Channels,
		SampleFormat: sampleformat,
	}

	var err error
	ps.stream, err = portaudio.NewStream(outStreamParams, float32(ps.audioFormat.SampleRate))
	if err != nil {
		slog.Error("failed to create stream", "error", err)
		return err
	}

	err = ps.stream.Open(ps.framesPerBuffer)
	if err != nil {
		slog.Error("failed to open stream", "error", err)
		return err
	}

	err = ps.stream.StartStream()
	if err != nil {
		slog.Error("failed to start stream", "error", err)
		return err
	}

	slog.Info("PortAudio stream initialized successfully")
	return nil
}

func (ps *portAudioSink) recreateStream(newFormat types.FrameFormat) error {
	// Close old stream to prevent resource leak
	if ps.stream != nil {
		if err := ps.stream.StopStream(); err != nil {
			slog.Warn("failed to stop old stream", "error", err)
		}
		if err := ps.stream.Close(); err != nil {
			slog.Warn("failed to close old stream", "error", err)
		}
	}

	ps.audioFormat = newFormat

	// Reinitialize with new format
	return ps.initializeStream()
}

func (ps *portAudioSink) writePacket(ctx context.Context, pkt audiosource.AudioSamplesPacket) error {
	err := ps.stream.Write(pkt.SamplesCount, pkt.Audio)
	if err != nil {
		// check if context was cancelled
		if ctx.Err() != nil {
			slog.Info("context cancelled", "error", ctx.Err())
			return nil
		}
		// Check for underflow errors (more robust than exact string match)
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "underflow") {
			slog.Warn("output underflow, continuing", "error", err)
			return nil
		}
		slog.Error("failed to write audio", "error", err)
		return err
	}
	return nil
}

func (ps *portAudioSink) Close(ctx context.Context) error {
	if ps.stream == nil {
		// Stream was never initialized
		return nil
	}

	slog.Info("closing PortAudio stream")

	if err := ps.stream.StopStream(); err != nil {
		slog.Error("failed to stop stream", "error", err)
		return err
	}

	if err := ps.stream.Close(); err != nil {
		slog.Error("failed to close stream", "error", err)
		return err
	}

	return nil
}
