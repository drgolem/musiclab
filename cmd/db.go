/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/dsp"
	"github.com/drgolem/sonet/cmd/scan"
	"github.com/drgolem/sonet/types"
)

// dbCmd represents the db command
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doDatabaseCmd,
}

func init() {
	rootCmd.AddCommand(dbCmd)

	dbCmd.Flags().Bool("scan", false, "scan folder for music files")
	dbCmd.Flags().String("music-root", "", "root folder of music collection")
}

func doDatabaseCmd(cmd *cobra.Command, args []string) {
	musicRoot, err := cmd.Flags().GetString("music-root")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	if _, err := os.Stat(musicRoot); os.IsNotExist(err) {
		fmt.Printf("path [%s] does not exist\n", musicRoot)
		return
	}

	ctx := context.Background()

	var wgProcess *errgroup.Group
	wgProcess, ctx = errgroup.WithContext(ctx)

	wgProcess.SetLimit(scan.MaxConcurrency)

	musicFileTypes := []types.FileFormatType{
		types.FileFormat_MP3,
		types.FileFormat_FLAC,
		types.FileFormat_OGG,
		types.FileFormat_WAV,
	}

	songsChan, foldersChan, cuesheetChan, err := scan.MusicDocWalker(ctx, musicRoot,
		wgProcess, musicFileTypes...)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	songDocChan := make(chan types.SongDocument, scan.MaxConcurrency)

	var wgSubProcess sync.WaitGroup

	wgSubProcess.Add(1)
	wgProcess.Go(func() error {
		defer wgSubProcess.Done()
		for sd := range songsChan {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			songDocChan <- sd
		}
		return nil
	})

	wgSubProcess.Add(1)
	wgProcess.Go(func() error {
		defer wgSubProcess.Done()
		for folder := range foldersChan {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			folderName := path.Base(folder.FilePath)
			sd := types.SongDocument{
				Type:       types.DocTypeFolder,
				FolderName: folderName,
				Ancestors:  folder.Ancestors,
			}
			//songDocs = append(songDocs, sd)
			songDocChan <- sd
		}
		return nil
	})

	wgSubProcess.Add(1)
	wgProcess.Go(func() error {
		defer wgSubProcess.Done()
		for ch := range cuesheetChan {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			ancestors := ch.Ancestors
			folderName := path.Base(ch.FilePath)
			sd := types.SongDocument{
				Type:       types.DocTypeCuesheet,
				FolderName: folderName,
				Ancestors:  ancestors,
			}

			//songDocs = append(songDocs, sd)
			songDocChan <- sd
		}
		return nil
	})

	wgProcess.Go(func() error {
		// wait for all subprocess finish
		// and close output channel
		wgSubProcess.Wait()
		close(songDocChan)
		return nil
	})

	mrd := types.SongDocument{
		Type:       types.DocTypeMusicRoot,
		FolderName: musicRoot,
	}
	//songDocs = append(songDocs, mrd)
	songDocChan <- mrd

	processSongs(ctx, songDocChan)
}

func processSongs(ctx context.Context, songDocChan <-chan types.SongDocument) {
	// database
	// songs: {idx, filePath}
	// hashes: {key: hash, value: (timestamp, song idx)}

	type SongHashLocator struct {
		Ts      int64
		SongIdx int64
	}

	songs := make([]string, 0)
	songHashes := make(map[uint64][]SongHashLocator)

	songIdx := int64(0)
	for sd := range songDocChan {
		if sd.Type != types.DocTypeSong {
			continue
		}
		fmt.Printf("%v\n", sd)

		inFileName := sd.Song.FilePath

		songs = append(songs, sd.Song.FilePath)

		audioData, err := audiosource.AudioSamplesFromFile(ctx, inFileName)
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			return
		}

		fmt.Printf("Spectrogram: %s\n", inFileName)
		fmt.Printf("Encoding: Signed 16bit\n")
		fmt.Printf("Sample Rate: %d\n", audioData.SampleRate)

		audioSamples := audioData.Audio
		sampleRate := audioData.SampleRate

		audioSamplesCopy := slices.Clone(audioSamples)

		//frameShift := 441 // 0.01 sec
		frameShift := 4410 // 0.1 sec
		frameSamples := 2048

		t0 := time.Now()
		stft := dsp.New(
			frameShift,
			frameSamples,
		)

		stftRes := stft.STFT(audioSamplesCopy)
		spectrogram, _ := dsp.SplitSpectrogram(stftRes)

		fmt.Printf("spectrogram %v\n", time.Since(t0))

		//sOut := printMatrixAsGnuplotFormat(spectrogram, sampleRate)
		//os.WriteFile(inFileName+".dat", []byte(sOut), 0644)

		//timeFreqPeaks := make([][]float64, 0)

		t1 := time.Now()
		for idx, sl := range spectrogram {
			peaks := octaveBinPeaks(sampleRate, 1.0, sl)
			//fmt.Printf("%d - %v\n", idx, peaks)
			//timeFreqPeaks = append(timeFreqPeaks, peaks)

			h := peaksHash(peaks)
			if h == 0 {
				continue
			}

			timePt := time.Duration(idx*frameShift*1000/sampleRate) * time.Millisecond
			//fmt.Printf("%d [%v] - %d\n", idx, timePt, h)

			if sh, ok := songHashes[h]; ok {
				sh = append(sh, SongHashLocator{
					Ts:      timePt.Milliseconds(),
					SongIdx: songIdx,
				})
				songHashes[h] = sh
			} else {
				sl := make([]SongHashLocator, 1)

				sl[0] = SongHashLocator{
					Ts:      timePt.Milliseconds(),
					SongIdx: songIdx,
				}

				songHashes[h] = sl
			}
		}
		fmt.Printf("octave bins %v\n", time.Since(t1))

		songIdx++
	}

	for h, v := range songHashes {
		fmt.Printf("Hash: %d\n", h)
		fmt.Printf("%v\n", v)
	}
}
