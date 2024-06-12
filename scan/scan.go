package scan

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/drgolem/go-cuesheet/cuesheet"
	"github.com/drgolem/musiclab/types"
)

const (
	idTagTitle  = "Title"
	idTagArtist = "Artist"
	idTagAlbum  = "Album/Movie/Show title"
)

const (
	MaxConcurrency = 8
)

type CueTrack struct {
	Title    string
	Artist   string
	Number   uint
	StartPos time.Duration
	Duration time.Duration
}

const (
	cueFramesPerSecond = 75
)

func frameToDuration(fd cuesheet.Frame) time.Duration {
	d := float32(fd) / cueFramesPerSecond

	dur, _ := time.ParseDuration(fmt.Sprintf("%0.3fs", d))
	return dur
}

type MusicTagDecoder interface {
	Decode(fileName string) (*types.SongInfo, error)
}

func ToSongDocument(si *types.SongInfo) types.SongDocument {
	//parent := ""
	dir := filepath.Dir(si.FilePath)
	ancestors := strings.Split(dir, "/")
	// first element - root / - empty string, remove it
	ancestors = ancestors[1:]
	//if len(ancestors) > 0 {
	//	parent = ancestors[len(ancestors)-1]
	//}

	//folderName := path.Base(dir)
	sd := types.SongDocument{
		Type: types.DocTypeSong,
		Song: si,
		//FolderName: folderName,
		//Folder: dir,
		//Parent:    parent,
		Ancestors: ancestors,
	}

	return sd
}
