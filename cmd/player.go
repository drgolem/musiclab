/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/drgolem/go-portaudio/portaudio"
	"github.com/spf13/cobra"

	"github.com/drgolem/musiclab/audioconsumer"
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
}

func doPlayerCmd(cmd *cobra.Command, args []string) {
	fileName, err := cmd.Flags().GetString("file")
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

	audioDataChan, audioFormat, closeFn, err := audiosource.MusicAudioProducer(ctx, fileName, framesPerBuffer)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer closeFn()

	fmt.Printf("Encoding: Signed 16bit\n")
	fmt.Printf("Sample Rate: %d\n", audioFormat.SampleRate)
	fmt.Printf("Channels: %d\n", audioFormat.NumChannels)

	deviceIdx := 1

	portaudio.Initialize()
	defer portaudio.Terminate()

	playFn, consumerCloseFn, err := audioconsumer.PortaudioConsumer(deviceIdx, framesPerBuffer, audioFormat, audioDataChan)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer consumerCloseFn()

	go func() {
		err := playFn(ctx)

		// notify cancel ctx
		cancelFn()

		if err != nil {
			fmt.Printf("ERR playFn: %v\n", err)
		}
	}()

	<-ctx.Done()
	fmt.Printf("done\n")
}
