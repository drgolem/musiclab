package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/drgolem/go-cuesheet/cuesheet"
	"github.com/drgolem/musiclab/types"
	"github.com/mewkiz/flac"
)

type CueTrackDecoder struct{}

func (d *CueTrackDecoder) Decode(file string) ([]types.SongInfo, error) {
	cueFile, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, file)
	}

	cue, err := cuesheet.ReadFile(cueFile)
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, file)
	}

	album := cue.Title

	if len(cue.File) != 1 {
		// support only cue sheet for albums
		return nil, nil
	}

	cueFileInfo := cue.File[0]
	musicFile := cueFileInfo.FileName
	musicDir := filepath.Dir(file)
	musicFileName := filepath.Join(musicDir, musicFile)
	musicFileExt := types.FileFormatType(filepath.Ext(musicFileName))

	if musicFileExt != types.FileFormat_FLAC {
		fmt.Printf("implement CUE for: %s (%s)\n", musicFileExt, file)
		return nil, nil
	}

	stream, err := flac.ParseFile(musicFileName)
	if err != nil {
		return nil, fmt.Errorf("ERR: %w, file: %s", err, file)
	}

	si := stream.Info
	song_length := float64(si.NSamples) / float64(si.SampleRate)
	albumDuration, _ := time.ParseDuration(fmt.Sprintf("%fs", song_length))

	tracks := make([]CueTrack, 0)
	for trIdx, tr := range cueFileInfo.Tracks {
		if tr.TrackDataType != "AUDIO" {
			continue
		}

		track := CueTrack{
			Title:  tr.Title,
			Artist: tr.Performer,
			Number: tr.TrackNumber,
		}

		if len(tr.Index) == 0 {
			fmt.Printf("invalid index: %s, track: %v\n", file, tr)
		} else if len(tr.Index) == 1 {
			track.StartPos = frameToDuration(tr.Index[0].Frame)
		} else {
			track.StartPos = frameToDuration(tr.Index[1].Frame)
		}

		if trIdx > 0 {
			tracks[trIdx-1].Duration = frameToDuration(tr.Index[0].Frame) - tracks[trIdx-1].StartPos
		}

		tracks = append(tracks, track)
	}
	if len(tracks) == 0 {
		fmt.Printf("no tracks in CUE: %s (%s)\n", musicFileExt, file)
		return nil, nil
	}
	// duration of last song
	tracks[len(tracks)-1].Duration = albumDuration - tracks[len(tracks)-1].StartPos

	stream.Close()

	songInfoCol := make([]types.SongInfo, 0)

	for _, track := range tracks {
		title := fmt.Sprintf("%02d - %s", track.Number, track.Title)
		songInfo := types.SongInfo{
			Title:      title,
			Artist:     track.Artist,
			Album:      album,
			FilePath:   musicFileName,
			FileFormat: musicFileExt,
			Duration:   track.Duration,
			StartPos:   track.StartPos,
			Format: types.FrameFormat{
				SampleRate:    int(si.SampleRate),
				Channels:      int(si.NChannels),
				BitsPerSample: int(si.BitsPerSample),
			},
		}
		songInfoCol = append(songInfoCol, songInfo)
	}
	return songInfoCol, nil
}
