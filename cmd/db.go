/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/dsp"
	"github.com/drgolem/musiclab/scan"
	"github.com/drgolem/musiclab/types"
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

type SongHashLocator struct {
	Ts      int64
	SongIdx int64
	Hash    uint64
}

type MatchPosLocator struct {
	CandidateTs  int64
	TsDiff       float64
	Hash         uint64
	SamplePosIdx int
	SongIdx      int64
}

func init() {
	rootCmd.AddCommand(dbCmd)

	dbCmd.Flags().Bool("scan", false, "scan folder for music files")
	dbCmd.Flags().String("music-root", "", "root folder of music collection")
	dbCmd.Flags().String("db", "spectr.db", "database file")
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

	musicDb, err := cmd.Flags().GetString("db")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	doDbScan, err := cmd.Flags().GetBool("scan")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	if doDbScan {
		scanMusic(musicRoot, musicDb)
	}

}

func scanMusic(musicRoot string, musicDb string) {
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

	processSongs(ctx, musicDb, songDocChan)
}

func processSongs(ctx context.Context, musicDb string, songDocChan <-chan types.SongDocument) {
	// database
	// songs: {idx, filePath}
	// hashes: {key: hash, value: (timestamp, song idx)}

	//opts := badger.DefaultOptions("/tmp/badger")
	opts := badger.DefaultOptions(musicDb)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	songs := make([]string, 0)
	songHashes := make(map[uint64][]SongHashLocator)

	frameShift := 441
	frameSamples := 2048

	var wgProcess *errgroup.Group
	wgProcess, ctx = errgroup.WithContext(ctx)

	//wgProcess.SetLimit(scan.MaxConcurrency)
	wgProcess.SetLimit(4)
	var hsMx sync.Mutex

	songIdx := int64(0)
	for sd := range songDocChan {
		if sd.Type != types.DocTypeSong {
			continue
		}
		fmt.Printf("%v\n", sd.Song.FilePath)

		inFileName := sd.Song.FilePath

		songs = append(songs, sd.Song.FilePath)

		idx := songIdx
		wgProcess.Go(
			func() error {
				fileSampleRate, refFileSpectr := getFileSpectrogram(ctx, inFileName, frameShift, frameSamples)

				sh := spectrToSongHashes(fileSampleRate, frameShift, idx, refFileSpectr)

				hsMx.Lock()
				defer hsMx.Unlock()

				for k, v := range sh {
					if sv, ok := songHashes[k]; !ok {
						songHashes[k] = v
					} else {
						sv = append(sv, v...)
						songHashes[k] = sv
					}
				}
				return nil
			})

		songIdx++
	}

	wgProcess.Wait()

	for idx, s := range songs {
		fmt.Printf("idx: %d - song: %s\n", idx, s)
	}

	t0 := time.Now()

	err = db.Update(func(txn *badger.Txn) error {
		prefix := []byte("hash")
		for k, v := range songHashes {
			binKey := make([]byte, 8)
			binary.LittleEndian.PutUint64(binKey, k)
			binKey = append(prefix, binKey...)
			binVal := arrSongLocatorsToBytes(v)
			err = txn.Set(binKey, binVal)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("ERR: %v", err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		prefix := []byte("song")
		for idx, song := range songs {
			binKey := make([]byte, 8)
			binary.LittleEndian.PutUint64(binKey, uint64(idx))
			binKey = append(prefix, binKey...)
			err = txn.Set(binKey, []byte(song))
		}
		return nil
	})
	if err != nil {
		log.Fatalf("ERR: %v", err)
	}

	fmt.Printf("db created in %v\n", time.Since(t0))
}

func arrSongLocatorsFromBytes(data []byte) []SongHashLocator {

	buf := bytes.NewBuffer(data)

	var n uint32
	binary.Read(buf, binary.LittleEndian, &n)

	lst := make([]SongHashLocator, 0)

	for i := uint32(0); i < n; i++ {
		var sh SongHashLocator
		binary.Read(buf, binary.LittleEndian, &sh.Ts)
		binary.Read(buf, binary.LittleEndian, &sh.SongIdx)
		binary.Read(buf, binary.LittleEndian, &sh.Hash)

		lst = append(lst, sh)
	}

	return lst
}

func arrSongLocatorsToBytes(lst []SongHashLocator) []byte {
	n := uint32(len(lst))

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, n)
	for _, sh := range lst {
		binary.Write(buf, binary.LittleEndian, sh.Ts)
		binary.Write(buf, binary.LittleEndian, sh.SongIdx)
		binary.Write(buf, binary.LittleEndian, sh.Hash)
	}

	return buf.Bytes()
}

func getFileSpectrogram(ctx context.Context,
	fileName string, frameShift int, frameSamples int) (int, [][]float64) {
	audioData, err := audiosource.AudioSamplesFromFile(ctx, fileName)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return 0, nil
	}

	fmt.Printf("Spectrogram: %s\n", fileName)

	audioSamples := audioData.Audio
	sampleRate := audioData.SampleRate

	t0 := time.Now()
	stft := dsp.New(
		frameShift,
		frameSamples,
	)

	stftRes := stft.STFT(audioSamples)
	spectrogram, _ := dsp.SplitSpectrogram(stftRes)

	fmt.Printf("spectrogram done in %v\n", time.Since(t0))
	return sampleRate, spectrogram
}
