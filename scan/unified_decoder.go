package scan

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/drgolem/audiokit/pkg/types"
	"github.com/drgolem/musictags/pkg/tags"
)

// UnifiedTagDecoder is a universal decoder that supports MP3, FLAC, OGG (Vorbis/Opus), and WAV.
// It wraps the musictags package for consistent metadata extraction across formats.
// Thread-safety: Each instance should be used by a single goroutine.
// Create a new instance per goroutine for concurrent processing.
type UnifiedTagDecoder struct{}

// Decode extracts metadata and audio format information from a music file.
// Automatically detects format based on file extension (.mp3, .flac, .ogg, .wav).
func (d *UnifiedTagDecoder) Decode(fileName string) (*types.SongInfo, error) {
	ext := strings.ToLower(filepath.Ext(fileName))

	var reader tags.Reader
	var fileFormat types.FileFormatType

	switch ext {
	case ".mp3":
		reader = tags.NewMP3Reader()
		fileFormat = types.FileFormatMP3
	case ".flac":
		reader = tags.NewFLACReader()
		fileFormat = types.FileFormatFLAC
	case ".ogg":
		reader = &tags.OggReader{}
		fileFormat = types.FileFormatOGG
	case ".wav":
		reader = &tags.WAVReader{}
		fileFormat = types.FileFormatWAV
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	metadata, err := reader.Read(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w, file: %s", err, fileName)
	}

	songInfo := metadataToSongInfo(metadata, fileFormat)
	return &songInfo, nil
}

func metadataToSongInfo(m *tags.Metadata, fileFormat types.FileFormatType) types.SongInfo {
	duration := time.Duration(m.Duration * float64(time.Second))

	si := types.SongInfo{
		Title:      m.Title,
		Artist:     m.Artist,
		Album:      m.Album,
		Genre:      m.Genre,
		Duration:   duration,
		FilePath:   m.FilePath,
		FileFormat: fileFormat,
		Format: types.FrameFormat{
			SampleRate:    m.SampleRate,
			Channels:      m.Channels,
			BitsPerSample: m.BitsPerSample,
		},
	}

	if m.HasReplayGain {
		si.RGEmbedded = &types.ReplayGainValues{
			TrackGain: m.ReplayGainTrackGain,
			TrackPeak: m.ReplayGainTrackPeak,
			AlbumGain: m.ReplayGainAlbumGain,
			AlbumPeak: m.ReplayGainAlbumPeak,
		}
	}

	return si
}
