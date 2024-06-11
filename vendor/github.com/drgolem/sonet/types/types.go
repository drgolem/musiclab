package types

import (
	"fmt"
	"path"
	"time"
)

type DisplayMode int

const (
	DisplayModePlayer  DisplayMode = 1
	DisplayModeBrowser DisplayMode = 2
)

type PlaylistItem struct {
	Id int
	SongInfo
}

type FileFormatType string

const (
	FileFormat_MP3  FileFormatType = ".mp3"
	FileFormat_FLAC FileFormatType = ".flac"
	FileFormat_OGG  FileFormatType = ".ogg"
	FileFormat_WAV  FileFormatType = ".wav"
	FileFormat_CUE  FileFormatType = ".cue"
)

type FrameFormat struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
}

type SongInfo struct {
	Title      string
	Artist     string
	Album      string
	Duration   time.Duration
	StartPos   time.Duration
	Format     FrameFormat
	FileFormat FileFormatType
	FilePath   string `json:"FilePath,omitempty" bson:"FilePath,omitempty" structs:"FilePath,omitempty"`
	ID         string
}

type SongLocation struct {
	ID   string
	Path string
}

type MusicDbDriverType string

const (
	MusicDb_Mongo    MusicDbDriverType = "mongodb"
	MusicDb_Couch    MusicDbDriverType = "couchdb"
	MusicDb_Json     MusicDbDriverType = "jsondb"
	MusicDb_Postgres MusicDbDriverType = "postgresdb"
)

type DocType string

const (
	DocTypeSong         DocType = "song"
	DocTypeSongLocation DocType = "songlocation"
	DocTypeFolder       DocType = "folder"
	DocTypeCuesheet     DocType = "cuesheet"
	DocTypeMusicRoot    DocType = "musicroot"
)

type SongDocument struct {
	Type       DocType
	Song       *SongInfo `json:"Song,omitempty" bson:"Song,omitempty" structs:"Song,omitempty"`
	FolderName string    `json:"FolderName,omitempty" bson:"FolderName,omitempty" structs:"FolderName,omitempty"`
	//Folder     string
	//Parent    string
	SongLocation *SongLocation `json:"SongLocation,omitempty" bson:"SongLocation,omitempty" structs:"SongLocation,omitempty"`
	Ancestors    []string      `json:"Ancestors,omitempty" bson:"Ancestors,omitempty" structs:"Ancestors,omitempty"`
}

func (sd SongDocument) FolderPath() string {
	parent := path.Join(sd.Ancestors...)
	if sd.Type == DocTypeSong {
		return parent
	}
	return path.Join(parent, sd.FolderName)
}

func (f *FrameFormat) String() string {
	return fmt.Sprintf("%d:%d:%d", f.SampleRate, f.Channels, f.BitsPerSample)
}

func (s *SongInfo) String() string {
	return fmt.Sprintf("Duration: [%s], Title: [%s], Artist: [%s], Album: [%s], Format: [%s], file: [%s]",
		s.Duration, s.Title, s.Artist, s.Album, s.Format.String(), s.FilePath)
}
