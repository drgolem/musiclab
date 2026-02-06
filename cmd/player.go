package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/drgolem/go-portaudio/portaudio"
	"github.com/drgolem/musiclab/audiosink"
	"github.com/drgolem/musiclab/audiosource"
)

// playerCmd represents the player command
var playerCmd = &cobra.Command{
	Use:   "play",
	Short: "play audio file",
	Long:  "play audio file",
	Run:   doPlayerCmd,
}

func init() {
	rootCmd.AddCommand(playerCmd)

	playerCmd.Flags().String("file", "", "file to play")
	playerCmd.Flags().String("start", "0", "start play at specified time")
	playerCmd.Flags().String("duration", "0", "duration of play (0 - play all)")
	playerCmd.Flags().Int("device", 1, "audio device index (1 for default)")

	playerCmd.MarkFlagRequired("file")
}

func doPlayerCmd(cmd *cobra.Command, args []string) {
	fileName, err := cmd.Flags().GetString("file")
	if err != nil {
		slog.Error("failed to get file flag", "error", err)
		return
	}
	if fileName == "" {
		slog.Error("file argument is required")
		return
	}
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		slog.Error("path does not exist", "path", fileName)
		return
	}

	startStr, err := cmd.Flags().GetString("start")
	if err != nil {
		slog.Error("failed to get start flag", "error", err)
		return
	}
	start, err := time.ParseDuration(startStr)
	if err != nil {
		slog.Error("invalid start time", "value", startStr, "error", err)
		return
	}
	if start < 0 {
		slog.Error("start time cannot be negative", "value", start)
		return
	}

	durtStr, err := cmd.Flags().GetString("duration")
	if err != nil {
		slog.Error("failed to get duration flag", "error", err)
		return
	}
	dur, err := time.ParseDuration(durtStr)
	if err != nil {
		slog.Error("invalid duration", "value", durtStr, "error", err)
		return
	}
	if dur < 0 {
		slog.Error("duration cannot be negative", "value", dur)
		return
	}

	deviceIdx, err := cmd.Flags().GetInt("device")
	if err != nil {
		slog.Error("invalid device index", "error", err)
		return
	}

	slog.Info("Playing audio file", "file", fileName)
	slog.Info("Press Ctrl-C to stop")

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	var stop context.CancelFunc
	ctx, stop = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	const framesPerBuffer = 2048

	audioStream, err := audiosource.NewMusicAudioProducer(ctx, fileName,
		audiosource.WithFramesPerBuffer(framesPerBuffer),
		audiosource.WithPlayStartPos(start),
		audiosource.WithPlayDuration(dur))
	if err != nil {
		slog.Error("failed to create audio producer", "error", err)
		return
	}
	defer audioStream.Close()

	audioFormat := audioStream.GetFormat()

	stat := audioStream.Status()
	slog.Info("Audio stream status", "status", stat)

	slog.Info("Audio format from file",
		"sample_rate", audioFormat.SampleRate,
		"channels", audioFormat.Channels,
		"bits_per_sample", audioFormat.BitsPerSample)

	if err := portaudio.Initialize(); err != nil {
		slog.Error("failed to initialize PortAudio", "error", err)
		return
	}
	defer portaudio.Terminate()

	sink, err := audiosink.NewPortAudioSink(deviceIdx,
		framesPerBuffer, audioStream.Stream())
	if err != nil {
		slog.Error("failed to create audio sink", "error", err)
		return
	}
	defer sink.Close(context.Background())

	go func() {
		err := sink.Play(ctx)

		// notify cancel ctx
		cancelFn()

		if err != nil {
			slog.Error("audio playback failed", "error", err)
		}
	}()

	go func(ctx context.Context) {
		// status reporter
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stat := audioStream.Status()
				slog.Info("Audio stream status", "status", stat)
			}
		}
	}(ctx)

	<-ctx.Done()
	slog.Info("Playback completed")
}
