package scan

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/drgolem/sonet/types"
	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/meta"
)

type FlacTagDecoder struct{}

func (d *FlacTagDecoder) Decode(file string) (*types.SongInfo, error) {
	stream, err := flac.ParseFile(file)
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, file)
	}
	defer stream.Close()

	artist := ""
	album := ""
	title := filepath.Base(file)

	for _, block := range stream.Blocks {
		switch body := block.Body.(type) {
		case *meta.VorbisComment:
			for _, tag := range body.Tags {
				tagName := strings.ToUpper(tag[0])
				tag[1] = strings.ReplaceAll(tag[1], "\x00", "")
				switch tagName {
				case "TITLE":
					title = strings.ToValidUTF8(tag[1], "")
				case "ALBUM":
					album = strings.ToValidUTF8(tag[1], "")
				case "ARTIST":
					artist = strings.ToValidUTF8(tag[1], "")
				}
			}
		}
	}

	si := stream.Info
	song_length := float64(si.NSamples) / float64(si.SampleRate)
	song_length = math.Floor(song_length)
	dur, _ := time.ParseDuration(fmt.Sprintf("%fs", song_length))

	songInfo := types.SongInfo{
		Title:      title,
		Artist:     artist,
		Album:      album,
		FilePath:   file,
		FileFormat: types.FileFormat_FLAC,
		Duration:   dur,
		Format: types.FrameFormat{
			SampleRate:    int(si.SampleRate),
			Channels:      int(si.NChannels),
			BitsPerSample: int(si.BitsPerSample),
		},
	}

	return &songInfo, nil
}
