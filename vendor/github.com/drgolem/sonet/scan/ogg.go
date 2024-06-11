package scan

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drgolem/sonet/types"
	"github.com/jfreymuth/oggvorbis"
)

type OggTagDecoder struct{}

func (d *OggTagDecoder) Decode(file string) (*types.SongInfo, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, file)
	}
	defer f.Close()

	reader := io.ReadSeeker(f)
	r, err := oggvorbis.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, file)
	}

	tags := r.CommentHeader().Comments

	artist := ""
	album := ""
	title := filepath.Base(file)

	for _, tag := range tags {
		tagVals := strings.Split(tag, "=")
		if len(tagVals) != 2 {
			continue
		}
		tagName := strings.ToUpper(tagVals[0])
		switch tagName {
		case "TITLE":
			title = tagVals[1]
		case "ALBUM":
			album = tagVals[1]
		case "ARTIST":
			artist = tagVals[1]
		}
	}

	sampleRate := r.SampleRate()
	channels := r.Channels()
	bitsPerSample := 12
	song_length := math.Floor(float64(r.Length()) / float64(sampleRate))
	dur, _ := time.ParseDuration(fmt.Sprintf("%fs", song_length))

	songInfo := types.SongInfo{
		Title:      title,
		Artist:     artist,
		Album:      album,
		FilePath:   file,
		FileFormat: types.FileFormat_OGG,
		Duration:   dur,
		Format: types.FrameFormat{
			SampleRate:    sampleRate,
			Channels:      channels,
			BitsPerSample: bitsPerSample,
		},
	}

	return &songInfo, nil
}
