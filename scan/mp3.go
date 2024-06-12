package scan

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bogem/id3v2/v2"
	"github.com/drgolem/go-mpg123/mpg123"
	"github.com/drgolem/musiclab/types"
)

type Mp3TagDecoder struct {
	muLibMp3 sync.Mutex
}

func (d *Mp3TagDecoder) Decode(fileName string) (*types.SongInfo, error) {
	var artist, album, title string
	d.muLibMp3.Lock()
	defer d.muLibMp3.Unlock()
	parseFields := []string{idTagArtist, idTagAlbum, idTagTitle}
	tag, err := id3v2.Open(fileName,
		id3v2.Options{
			Parse:       true,
			ParseFrames: parseFields,
		})
	if err == id3v2.ErrUnsupportedVersion {
		title = filepath.Base(fileName)
	} else if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, fileName)
	} else {
		artist = tag.GetTextFrame(tag.CommonID(idTagArtist)).Text
		album = tag.GetTextFrame(tag.CommonID(idTagAlbum)).Text
		title = tag.GetTextFrame(tag.CommonID(idTagTitle)).Text

		title = strings.ReplaceAll(title, "\x00", "")
		album = strings.ReplaceAll(album, "\x00", "")
		artist = strings.ReplaceAll(artist, "\x00", "")
	}

	tag.Close()

	//	f, err := os.Open(fileName)
	//	if err != nil {
	//		return nil, fmt.Errorf("ERR: %w, file: %s", err, fileName)
	//	}
	//	defer f.Close()
	//	m, err := musictag.ReadFrom(f)
	//	if err != nil {
	//		fmt.Printf("ERR: %v, file: %s\n", err, fileName)
	//		return nil, fmt.Errorf("ERR: %w, file: %s", err, fileName)
	//	}
	//	log.Print(m.Format()) // The detected format.
	//	log.Print(m.Title())  // The title of the track (see Metadata interface for more details).

	mp3Decoder, err := mpg123.NewDecoder("")
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, fileName)
	}
	err = mp3Decoder.Param(mpg123.ADD_FLAGS, mpg123.QUIET, 1)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return nil, fmt.Errorf("ERR: %w, file: %s", err, fileName)
	}

	err = mp3Decoder.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, fileName)
	}

	sampleRate, channels, enc := mp3Decoder.GetFormat()
	if channels == 0 {
		return nil, fmt.Errorf("ERR: invalid number of channels, file: %s", fileName)
	}
	bitsPerSample := mpg123.GetEncodingBitsPerSample(enc)
	curr_sample := mp3Decoder.GetLengthInPCMFrames()

	mp3Decoder.Close()
	mp3Decoder.Delete()

	song_length := float64(curr_sample) / float64(sampleRate)
	song_length = math.Floor(song_length)
	dur, _ := time.ParseDuration(fmt.Sprintf("%fs", song_length))

	songInfo := types.SongInfo{
		Title:      title,
		Artist:     artist,
		Album:      album,
		FilePath:   fileName,
		FileFormat: types.FileFormat_MP3,
		Duration:   dur,
		Format: types.FrameFormat{
			SampleRate:    int(sampleRate),
			Channels:      channels,
			BitsPerSample: bitsPerSample,
		},
	}

	return &songInfo, nil
}
