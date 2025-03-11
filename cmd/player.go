/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/drgolem/go-portaudio/portaudio"
	"github.com/spf13/cobra"

	"github.com/drgolem/musiclab/audiosink"
	"github.com/drgolem/musiclab/audiosource"
)

// playerCmd represents the player command
var playerCmd = &cobra.Command{
	Use:   "play",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doPlayerCmd,
}

func init() {
	rootCmd.AddCommand(playerCmd)

	playerCmd.Flags().String("file", "", "file to play")
	playerCmd.Flags().String("start", "0", "start play at specified time")
	playerCmd.Flags().String("duration", "0", "duration of play (0 - play all)")
}

func doPlayerCmd(cmd *cobra.Command, args []string) {
	fileName, err := cmd.Flags().GetString("file")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		fmt.Printf("path [%s] does not exist\n", fileName)
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

	fmt.Printf("Playing: %s\n", fileName)
	fmt.Printf("Press Ctrl-C to stop.\n")

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	const framesPerBuffer = 2048

	audioStream, err := audiosource.MusicAudioProducer(ctx, fileName,
		audiosource.WithFramesPerBuffer(framesPerBuffer),
		audiosource.WithPlayStartPos(start),
		audiosource.WithPlayDuration(dur))
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer audioStream.Close()

	audioFormat := audioStream.GetFormat()

	fmt.Printf("Encoding: Signed 16bit\n")
	fmt.Printf("Sample Rate: %d\n", audioFormat.SampleRate)
	fmt.Printf("Channels: %d\n", audioFormat.Channels)
	deviceIdx := 1

	portaudio.Initialize()
	defer portaudio.Terminate()

	sink, err := audiosink.NewPortAudioSink(deviceIdx,
		framesPerBuffer, audioStream.Stream())
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer sink.Close(ctx)

	go func() {
		err := sink.Play(ctx)

		// notify cancel ctx
		cancelFn()

		if err != nil {
			fmt.Printf("ERR playFn: %v\n", err)
		}
	}()

	<-ctx.Done()
	fmt.Printf("done\n")
}
