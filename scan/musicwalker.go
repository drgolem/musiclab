package scan

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/drgolem/audiokit/pkg/types"
	"github.com/karrick/godirwalk"
	"golang.org/x/sync/errgroup"
)

type Folder struct {
	FilePath  string
	Ancestors []string
}

type CueSheet struct {
	FilePath  string
	Ancestors []string
}

// MusicDocWalker recursively walks a music directory and extracts metadata from audio files.
// Uses DefaultScanConfig(). For custom configuration, use MusicDocWalkerWithConfig.
func MusicDocWalker(ctx context.Context,
	musicRoot string,
	wgProcess *errgroup.Group,
	fileTypes ...types.FileFormatType,
) (<-chan types.SongDocument, <-chan Folder, <-chan CueSheet, error) {
	return MusicDocWalkerWithConfig(ctx, musicRoot, wgProcess, DefaultScanConfig(), fileTypes...)
}

// MusicDocWalkerWithConfig is like MusicDocWalker but accepts a custom ScanConfig.
func MusicDocWalkerWithConfig(ctx context.Context,
	musicRoot string,
	wgProcess *errgroup.Group,
	config ScanConfig,
	fileTypes ...types.FileFormatType,
) (<-chan types.SongDocument, <-chan Folder, <-chan CueSheet, error) {
	concurrency := config.MaxConcurrency
	filesChan := make(chan string, concurrency)
	foldersChan := make(chan Folder, concurrency)
	cuesheetChan := make(chan CueSheet, concurrency)

	if len(fileTypes) == 0 {
		fileTypes = []types.FileFormatType{
			types.FileFormatMP3, types.FileFormatFLAC, types.FileFormatOGG, types.FileFormatWAV,
		}
	}

	wgProcess.Go(
		func() error {
			err := godirwalk.Walk(musicRoot, &godirwalk.Options{
				Callback: func(osPathname string, de *godirwalk.Dirent) error {
					if strings.Contains(osPathname, "@eaDir") {
						return filepath.SkipDir
					}

					if de.IsDir() {
						if osPathname == musicRoot {
							return nil
						}

						ancestors := PathToAncestors(musicRoot, osPathname)

						select {
						case foldersChan <- Folder{
							FilePath:  osPathname,
							Ancestors: ancestors,
						}:
						case <-ctx.Done():
							return ctx.Err()
						}

						return nil
					}

					if !de.IsRegular() {
						return nil
					}

					// Skip hidden files
					if config.SkipHiddenFiles && strings.HasPrefix(de.Name(), ".") {
						return nil
					}

					ext := types.FileFormatType(filepath.Ext(de.Name()))

					reqType := slices.Contains(fileTypes, ext)
					if reqType {
						// Quick file size check to skip invalid/tiny files
						if info, err := os.Stat(osPathname); err == nil && info.Size() < config.MinFileSize {
							return nil
						}

						select {
						case filesChan <- osPathname:
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					if ext == types.FileFormatCUE {
						cueFile := osPathname
						ancestors := PathToAncestors(musicRoot, osPathname)

						ch := CueSheet{
							FilePath:  cueFile,
							Ancestors: ancestors,
						}

						select {
						case cuesheetChan <- ch:
						case <-ctx.Done():
							return ctx.Err()
						}

						select {
						case filesChan <- cueFile:
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					return nil
				},
				Unsorted: true,
			})

			slog.Info("Completed music directory walk")
			close(filesChan)
			close(foldersChan)
			close(cuesheetChan)

			return err
		},
	)

	songsChan := make(chan types.SongDocument, concurrency)

	wgProcess.Go(
		func() error {
			wgSubProcess, ctxSub := errgroup.WithContext(ctx)
			wgSubProcess.SetLimit(concurrency)

			// Metrics counters
			var filesProcessed, mp3Count, flacCount, oggCount, wavCount, cueCount atomic.Int64
			startTime := time.Now()

			slog.Info("Starting music file processing",
				"root", musicRoot,
				"maxConcurrency", concurrency)
		LOOP:
			for file := range filesChan {
				select {
				case <-ctxSub.Done():
					break LOOP
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				ext := types.FileFormatType(filepath.Ext(file))
				switch ext {
				case types.FileFormatMP3, types.FileFormatFLAC, types.FileFormatOGG, types.FileFormatWAV:
					wgSubProcess.Go(func() (err error) {
						defer func() {
							if r := recover(); r != nil {
								err = fmt.Errorf("panic processing %s: %v", file, r)
							}
						}()
						select {
						case <-ctxSub.Done():
							return ctx.Err()
						default:
						}

						tagDecoder := UnifiedTagDecoder{}
						songInfo, err := tagDecoder.Decode(file)
						if err != nil {
							slog.Error("Failed to decode file",
								"file", file, "error", err)
							return err
						}
						if songInfo != nil {
							sd := ToSongDocument(musicRoot, songInfo)
							select {
							case songsChan <- sd:
								filesProcessed.Add(1)
								switch ext {
								case types.FileFormatMP3:
									mp3Count.Add(1)
								case types.FileFormatFLAC:
									flacCount.Add(1)
								case types.FileFormatOGG:
									oggCount.Add(1)
								case types.FileFormatWAV:
									wavCount.Add(1)
								}
							case <-ctxSub.Done():
								return ctx.Err()
							}
						}
						return nil
					})

				case types.FileFormatCUE:
					wgSubProcess.Go(func() (err error) {
						defer func() {
							if r := recover(); r != nil {
								err = fmt.Errorf("panic processing CUE %s: %v", file, r)
							}
						}()
						tagDecoder := CueTrackDecoder{}
						tracks, err := tagDecoder.Decode(file)
						if err != nil {
							slog.Error("Failed to decode CUE file",
								"file", file, "error", err)
							return err
						}
						for _, songInfo := range tracks {
							ancestors := PathToAncestors(musicRoot, file)

							cueBaseFile := path.Base(file)
							ancestors = append(ancestors, cueBaseFile)

							si := songInfo
							si.FilePath = types.ToRelativePath(musicRoot, si.FilePath)
							sd := types.SongDocument{
								Type:      types.DocTypeSong,
								Song:      &si,
								Ancestors: ancestors,
							}

							select {
							case songsChan <- sd:
								filesProcessed.Add(1)
							case <-ctxSub.Done():
								return ctx.Err()
							}
						}
						if len(tracks) > 0 {
							cueCount.Add(1)
						}
						return nil
					})

				default:
					slog.Warn("unknown file type", "ext", ext, "file", file)
				}
			}

			err := wgSubProcess.Wait()

			close(songsChan)

			// Log processing statistics
			duration := time.Since(startTime)
			slog.Info("Completed music file processing",
				"totalFiles", filesProcessed.Load(),
				"mp3Files", mp3Count.Load(),
				"flacFiles", flacCount.Load(),
				"oggFiles", oggCount.Load(),
				"wavFiles", wavCount.Load(),
				"cueSheets", cueCount.Load(),
				"duration", duration.Round(time.Millisecond).String(),
				"filesPerSecond", fmt.Sprintf("%.1f", float64(filesProcessed.Load())/duration.Seconds()))

			return err
		})

	return songsChan, foldersChan, cuesheetChan, nil
}
