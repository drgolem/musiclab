package scan

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/drgolem/sonet/types"
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

func MusicDocWalker(ctx context.Context,
	musicRoot string,
	wgProcess *errgroup.Group,
	fileTypes ...types.FileFormatType,
) (<-chan types.SongDocument, <-chan Folder, <-chan CueSheet, error) {
	filesChan := make(chan string, MaxConcurrency)
	foldersChan := make(chan Folder, MaxConcurrency)
	cuesheetChan := make(chan CueSheet, MaxConcurrency)

	if len(fileTypes) == 0 {
		fileTypes = []types.FileFormatType{
			types.FileFormat_MP3, types.FileFormat_FLAC, types.FileFormat_OGG,
		}
	}

	wgProcess.Go(
		func() error {
			err := godirwalk.Walk(musicRoot, &godirwalk.Options{
				Callback: func(osPathname string, de *godirwalk.Dirent) error {
					if strings.Contains(osPathname, "@eaDir") {
						// skip this
						return filepath.SkipDir
					}

					if de.IsDir() {
						dir := osPathname
						ancestors := strings.Split(dir, "/")
						// first element - root / - empty string, remove it
						ancestors = ancestors[1:]
						ancestors = ancestors[:len(ancestors)-1]

						select {
						case foldersChan <- Folder{FilePath: dir, Ancestors: ancestors}:
							// fmt.Printf("POST: %s\n", dir)
						case <-ctx.Done():
							return ctx.Err()
						}

						return nil
					}

					if !de.IsRegular() {
						return nil
					}

					ext := types.FileFormatType(filepath.Ext(de.Name()))

					reqType := slices.Contains(fileTypes, ext)
					if reqType {
						// MP3, FLAC, OGG supported
						select {
						case filesChan <- osPathname:
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					if ext == types.FileFormat_CUE {
						cueFile := osPathname
						ancestors := strings.Split(cueFile, "/")
						// first element - root / - empty string, remove it
						ancestors = ancestors[1:]
						ch := CueSheet{
							FilePath:  cueFile,
							Ancestors: ancestors[:len(ancestors)-1],
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

			// done dir walk, close all channels
			fmt.Println("Done file walk")
			close(filesChan)
			close(foldersChan)
			close(cuesheetChan)

			return err
		},
	)

	songsChan := make(chan types.SongDocument, MaxConcurrency)

	var muLibFlac sync.Mutex
	var muLibOgg sync.Mutex
	var muLibCue sync.Mutex

	mp3TagDecoder := Mp3TagDecoder{}

	wgProcess.Go(
		func() error {
			wgSubProcess, ctxSub := errgroup.WithContext(ctx)
			wgSubProcess.SetLimit(MaxConcurrency)
		LOOP:
			for file := range filesChan {
				select {
				case <-ctxSub.Done():
					break LOOP
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				file := file

				ext := types.FileFormatType(filepath.Ext(file))
				switch ext {
				case types.FileFormat_MP3:
					wgSubProcess.Go(func() error {
						select {
						case <-ctxSub.Done():
							return ctx.Err()
						default:
						}

						songInfo, err := mp3TagDecoder.Decode(file)
						if err != nil {
							return err
						}
						if songInfo != nil {
							sd := ToSongDocument(songInfo)
							select {
							case songsChan <- sd:
							case <-ctxSub.Done():
								return ctx.Err()
							}
						}
						return nil
					})

				case types.FileFormat_FLAC:
					wgSubProcess.Go(func() error {
						muLibFlac.Lock()
						defer muLibFlac.Unlock()

						tagDecoder := FlacTagDecoder{}
						songInfo, err := tagDecoder.Decode(file)
						if err != nil {
							return err
						}
						if songInfo != nil {
							sd := ToSongDocument(songInfo)
							select {
							case songsChan <- sd:
							case <-ctxSub.Done():
								return ctx.Err()
							}
						}
						return nil
					})

				case types.FileFormat_OGG:
					wgSubProcess.Go(func() error {
						muLibOgg.Lock()
						defer muLibOgg.Unlock()

						tagDecoder := OggTagDecoder{}
						songInfo, err := tagDecoder.Decode(file)
						if err != nil {
							return err
						}
						if songInfo != nil {
							sd := ToSongDocument(songInfo)
							select {
							case songsChan <- sd:
							case <-ctxSub.Done():
								return ctx.Err()
							}
						}
						return nil
					})

				case types.FileFormat_WAV:
					wgSubProcess.Go(func() error {
						// TODO: process wav
						songInfo := &types.SongInfo{
							FilePath: file,
						}
						sd := ToSongDocument(songInfo)
						select {
						case songsChan <- sd:
						case <-ctxSub.Done():
							return ctx.Err()
						}
						return nil
					})

				case types.FileFormat_CUE:
					wgSubProcess.Go(func() error {
						muLibCue.Lock()
						defer muLibCue.Unlock()
						muLibFlac.Lock()
						defer muLibFlac.Unlock()

						tagDecoder := CueTrackDecoder{}
						tracks, err := tagDecoder.Decode(file)
						if err != nil {
							return err
						}
						for _, songInfo := range tracks {
							//parent := ""
							dir := file
							ancestors := strings.Split(dir, "/")
							// first element - root / - empty string, remove it
							ancestors = ancestors[1:]
							//if len(ancestors) > 0 {
							//	parent = ancestors[len(ancestors)-1]
							//}

							si := songInfo
							sd := types.SongDocument{
								Type: types.DocTypeSong,
								Song: &si,
								//Folder: dir,
								//Parent:    parent,
								Ancestors: ancestors,
							}

							select {
							case songsChan <- sd:
							case <-ctxSub.Done():
								return ctx.Err()
							}
							// fmt.Printf("CUE TRACK: %#v\n", sd)
							// fmt.Printf("CUE TRACK: %s\n", sd.Song.Title)
						}
						return nil
					})

				default:
					fmt.Printf("unknown file type: %v\n", ext)
				}
			}

			err := wgSubProcess.Wait()

			close(songsChan)

			return err
		})

	return songsChan, foldersChan, cuesheetChan, nil
}
