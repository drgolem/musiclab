/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
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
	"github.com/drgolem/audiokit/pkg/types"
)

// dbCmd represents the db command
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Build a spectral hash database from a music folder",
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
		slog.Error("failed to get music-root flag", "error", err)
		return
	}
	if _, err := os.Stat(musicRoot); os.IsNotExist(err) {
		slog.Error("path does not exist", "path", musicRoot)
		return
	}

	musicDb, err := cmd.Flags().GetString("db")
	if err != nil {
		slog.Error("failed to get db flag", "error", err)
		return
	}

	doDbScan, err := cmd.Flags().GetBool("scan")
	if err != nil {
		slog.Error("failed to get scan flag", "error", err)
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
		types.FileFormatMP3,
		types.FileFormatFLAC,
		types.FileFormatOGG,
		types.FileFormatWAV,
	}

	songsChan, foldersChan, cuesheetChan, err := scan.MusicDocWalker(ctx, musicRoot,
		wgProcess, musicFileTypes...)
	if err != nil {
		slog.Error("failed to walk music directory", "error", err)
		return
	}

	songDocChan := fanInSongDocuments(ctx, wgProcess, songsChan, foldersChan, cuesheetChan)

	mrd := types.SongDocument{
		Type:       types.DocTypeMusicRoot,
		FolderName: musicRoot,
	}
	songDocChan <- mrd

	processSongs(ctx, musicDb, songDocChan)
}

func fanInSongDocuments(ctx context.Context, wgProcess *errgroup.Group,
	songsChan <-chan types.SongDocument,
	foldersChan <-chan scan.Folder,
	cuesheetChan <-chan scan.CueSheet,
) chan types.SongDocument {
	songDocChan := make(chan types.SongDocument, scan.MaxConcurrency)
	var wgSubProcess sync.WaitGroup

	wgSubProcess.Add(1)
	wgProcess.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in songs goroutine: %v", r)
			}
		}()
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
	wgProcess.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in folders goroutine: %v", r)
			}
		}()
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
			songDocChan <- sd
		}
		return nil
	})

	wgSubProcess.Add(1)
	wgProcess.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in cuesheet goroutine: %v", r)
			}
		}()
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
			songDocChan <- sd
		}
		return nil
	})

	wgProcess.Go(func() error {
		wgSubProcess.Wait()
		close(songDocChan)
		return nil
	})

	return songDocChan
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
		slog.Error("failed to open database", "error", err)
		return
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
			func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic processing %s: %v", inFileName, r)
					}
				}()
				fileSampleRate, refFileSpectr := getFileSpectrogram(inFileName, frameShift, frameSamples)

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
			binVal, err := arrSongLocatorsToBytes(v)
			if err != nil {
				return err
			}
			if err := txn.Set(binKey, binVal); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("failed to store hashes in database", "error", err)
		return
	}

	err = db.Update(func(txn *badger.Txn) error {
		prefix := []byte("song")
		for idx, song := range songs {
			binKey := make([]byte, 8)
			binary.LittleEndian.PutUint64(binKey, uint64(idx))
			binKey = append(prefix, binKey...)
			if err := txn.Set(binKey, []byte(song)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("failed to store songs in database", "error", err)
		return
	}

	fmt.Printf("db created in %v\n", time.Since(t0))
}

func arrSongLocatorsFromBytes(data []byte) ([]SongHashLocator, error) {

	buf := bytes.NewBuffer(data)

	var n uint32
	if err := binary.Read(buf, binary.LittleEndian, &n); err != nil {
		return nil, fmt.Errorf("reading count: %w", err)
	}

	lst := make([]SongHashLocator, 0, n)

	for i := uint32(0); i < n; i++ {
		var sh SongHashLocator
		if err := binary.Read(buf, binary.LittleEndian, &sh.Ts); err != nil {
			return nil, fmt.Errorf("reading Ts: %w", err)
		}
		if err := binary.Read(buf, binary.LittleEndian, &sh.SongIdx); err != nil {
			return nil, fmt.Errorf("reading SongIdx: %w", err)
		}
		if err := binary.Read(buf, binary.LittleEndian, &sh.Hash); err != nil {
			return nil, fmt.Errorf("reading Hash: %w", err)
		}

		lst = append(lst, sh)
	}

	return lst, nil
}

func arrSongLocatorsToBytes(lst []SongHashLocator) ([]byte, error) {
	n := uint32(len(lst))

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, n); err != nil {
		return nil, fmt.Errorf("writing count: %w", err)
	}
	for _, sh := range lst {
		if err := binary.Write(buf, binary.LittleEndian, sh.Ts); err != nil {
			return nil, fmt.Errorf("writing Ts: %w", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, sh.SongIdx); err != nil {
			return nil, fmt.Errorf("writing SongIdx: %w", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, sh.Hash); err != nil {
			return nil, fmt.Errorf("writing Hash: %w", err)
		}
	}

	return buf.Bytes(), nil
}

func getFileSpectrogram(
	fileName string, frameShift int, frameSamples int) (int, [][]float64) {
	audioData, err := audiosource.AudioSamplesFromFile(fileName)
	if err != nil {
		slog.Error("failed to read audio samples", "file", fileName, "error", err)
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
