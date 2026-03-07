package scan

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/drgolem/audiokit/pkg/types"
	"github.com/drgolem/go-cuesheet/cuesheet"
	"github.com/mewkiz/flac"
)

// CueTrackDecoder extracts metadata from CUE sheet files.
// Thread-safety: Each instance should be used by a single goroutine.
// Create a new instance per goroutine for concurrent processing.
//
// Supports both single-file and multi-file CUE sheets:
//   - Single-file: All tracks in one audio file with different start positions
//   - Multi-file: Tracks distributed across multiple audio files (e.g., CD1.flac, CD2.flac)
type CueTrackDecoder struct{}

// Decode extracts track information from a CUE sheet file.
// Returns multiple SongInfo entries (one per track) for a single or multi-file CUE sheet.
func (d *CueTrackDecoder) Decode(file string) ([]types.SongInfo, error) {
	cueFile, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("file %s: %w", file, err)
	}
	defer cueFile.Close()

	cue, err := cuesheet.ReadFile(cueFile)
	if err != nil {
		return nil, fmt.Errorf("file %s: %w", file, err)
	}

	album := cue.Title
	musicDir := filepath.Dir(file)

	if len(cue.File) == 0 {
		slog.Warn("no files found in CUE sheet", "file", file)
		return nil, nil
	}

	// Process all files in CUE sheet (supports both single-file and multi-file)
	var allTracks []CueTrack

	for _, cueFileInfo := range cue.File {
		musicFile := cueFileInfo.FileName
		musicFileName := filepath.Join(musicDir, musicFile)
		musicFileExt := types.FileFormatType(filepath.Ext(musicFileName))

		if musicFileExt != types.FileFormatFLAC {
			slog.Warn("CUE sheet not yet supported for format",
				"format", musicFileExt,
				"file", file,
				"audioFile", musicFileName)
			continue
		}

		fileDuration, format, err := d.getAudioFileInfo(musicFileName)
		if err != nil {
			slog.Warn("failed to parse audio file for CUE sheet",
				"error", err,
				"cueFile", file,
				"audioFile", musicFileName)
			continue
		}

		tracks := d.extractTracksFromFile(cueFileInfo, musicFileName, musicFileExt, fileDuration, format)
		allTracks = append(allTracks, tracks...)
	}

	if len(allTracks) == 0 {
		slog.Warn("no audio tracks found in CUE file", "file", file)
		return nil, nil
	}

	songInfoCol := make([]types.SongInfo, 0, len(allTracks))
	for _, track := range allTracks {
		title := fmt.Sprintf("%02d - %s", track.Number, track.Title)
		songInfo := types.SongInfo{
			Title:      title,
			Artist:     track.Artist,
			Album:      album,
			FilePath:   track.FilePath,
			FileFormat: track.FileFormat,
			Duration:   track.Duration,
			StartPos:   track.StartPos,
			Format:     track.Format,
		}
		songInfoCol = append(songInfoCol, songInfo)
	}

	return songInfoCol, nil
}

// getAudioFileInfo retrieves duration and format information from a FLAC file.
func (d *CueTrackDecoder) getAudioFileInfo(musicFileName string) (time.Duration, types.FrameFormat, error) {
	stream, err := flac.ParseFile(musicFileName)
	if err != nil {
		return 0, types.FrameFormat{}, fmt.Errorf("failed to parse FLAC file: %w", err)
	}
	defer stream.Close()

	si := stream.Info
	songLength := float64(si.NSamples) / float64(si.SampleRate)
	duration, err := time.ParseDuration(fmt.Sprintf("%fs", songLength))
	if err != nil {
		return 0, types.FrameFormat{}, fmt.Errorf("failed to parse duration: %w", err)
	}

	format := types.FrameFormat{
		SampleRate:    int(si.SampleRate),
		Channels:      int(si.NChannels),
		BitsPerSample: int(si.BitsPerSample),
	}

	return duration, format, nil
}

// extractTracksFromFile extracts tracks from a single audio file entry in the CUE sheet.
func (d *CueTrackDecoder) extractTracksFromFile(
	cueFileInfo cuesheet.File,
	musicFileName string,
	musicFileExt types.FileFormatType,
	fileDuration time.Duration,
	format types.FrameFormat,
) []CueTrack {
	tracks := make([]CueTrack, 0)

	for _, tr := range cueFileInfo.Tracks {
		if tr.TrackDataType != "AUDIO" {
			continue
		}

		track := CueTrack{
			Title:      tr.Title,
			Artist:     tr.Performer,
			Number:     tr.TrackNumber,
			FilePath:   musicFileName,
			FileFormat: musicFileExt,
			Format:     format,
		}

		if len(tr.Index) == 0 {
			slog.Warn("invalid index in CUE file",
				"audioFile", musicFileName,
				"track", tr)
		} else if len(tr.Index) == 1 {
			track.StartPos = frameToDuration(tr.Index[0].Frame)
		} else {
			// Use INDEX 01 (the actual start), not INDEX 00 (pre-gap)
			track.StartPos = frameToDuration(tr.Index[1].Frame)
		}

		tracks = append(tracks, track)
	}

	if len(tracks) == 0 {
		return tracks
	}

	// Calculate track durations
	for i := 0; i < len(tracks)-1; i++ {
		tracks[i].Duration = tracks[i+1].StartPos - tracks[i].StartPos
	}
	// Last track duration extends to the end of the file
	tracks[len(tracks)-1].Duration = fileDuration - tracks[len(tracks)-1].StartPos

	return tracks
}
