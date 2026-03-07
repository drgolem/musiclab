package scan

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/drgolem/go-cuesheet/cuesheet"
	"github.com/drgolem/audiokit/pkg/types"
)

const (
	idTagTitle  = "Title"
	idTagArtist = "Artist"
	idTagAlbum  = "Album/Movie/Show title"
)

const (
	// MaxConcurrency controls parallel file decoding operations.
	MaxConcurrency = 8

	// MinFileSizeBytes filters out files smaller than this threshold.
	// Audio files are typically >1KB. This skips corrupted/incomplete files.
	MinFileSizeBytes = 1024
)

// ScanConfig holds configuration options for music file scanning.
type ScanConfig struct {
	// MaxConcurrency controls parallel file decoding (default: MaxConcurrency constant)
	MaxConcurrency int
	// MinFileSize skips files smaller than this (default: MinFileSizeBytes constant)
	MinFileSize int64
	// SkipHiddenFiles skips files/folders starting with '.' (default: true)
	SkipHiddenFiles bool
}

// DefaultScanConfig returns a configuration with sensible defaults.
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		MaxConcurrency:  MaxConcurrency,
		MinFileSize:     MinFileSizeBytes,
		SkipHiddenFiles: true,
	}
}

type CueTrack struct {
	Title      string
	Artist     string
	Number     uint
	StartPos   time.Duration
	Duration   time.Duration
	FilePath   string              // Path to the audio file containing this track
	FileFormat types.FileFormatType // Audio file format (.flac, .mp3, etc.)
	Format     types.FrameFormat   // Audio format details (sample rate, channels, etc.)
}

const (
	cueFramesPerSecond = 75
)

func frameToDuration(fd cuesheet.Frame) time.Duration {
	d := float32(fd) / cueFramesPerSecond

	dur, err := time.ParseDuration(fmt.Sprintf("%0.3fs", d))
	if err != nil {
		return 0
	}
	return dur
}

// MusicTagDecoder is the interface for extracting metadata from audio files.
// Implementations should be safe to use concurrently by creating separate
// instances per goroutine (not sharing instances across goroutines).
type MusicTagDecoder interface {
	Decode(fileName string) (*types.SongInfo, error)
}

// PathToAncestors converts a file path to a slice of ancestor directory names
// relative to musicRoot. Thread-safe (no shared state).
func PathToAncestors(musicRoot string, path string) []string {
	if path == musicRoot {
		return nil
	}
	path = filepath.Dir(path)
	relPath, err := filepath.Rel(musicRoot, path)
	if err != nil || relPath == "." {
		return nil
	}
	ancestors := strings.Split(relPath, string(filepath.Separator))
	result := make([]string, 0, len(ancestors))
	for _, a := range ancestors {
		if a != "" && a != "." {
			result = append(result, a)
		}
	}
	return result
}

// ToSongDocument converts a SongInfo to a SongDocument with ancestor hierarchy.
// Ancestors are computed from the absolute path relative to musicRoot,
// then FilePath is converted to a relative path for storage.
func ToSongDocument(musicRoot string, si *types.SongInfo) types.SongDocument {
	ancestors := PathToAncestors(musicRoot, si.FilePath)

	si.FilePath = types.ToRelativePath(musicRoot, si.FilePath)

	sd := types.SongDocument{
		Type:      types.DocTypeSong,
		Song:      si,
		Ancestors: ancestors,
	}

	return sd
}
