package types

import (
	"time"
)

type MusicPlayerClient interface {
	Close()

	Reconnect() error

	Status() (Attrs, error)

	Ping() error

	WalkMusicDir(parentPath string) ([]SongDocument, error)

	GetPlaylist() ([]PlaylistItem, error)

	GetCurrentSongId() int

	PlayID(id int) error

	PlayPos(pos int) error

	DeleteID(id int) error

	Pause(pause bool) error

	Stop() error

	Next() error

	Clear() error

	Add(sd SongDocument) error

	UpdateDatabase(uri string) error

	CurrentSong() (Attrs, error)

	SeekCur(d time.Duration, relative bool) error

	SetVolume(vol int) error

	NotifyChangeEvents() <-chan bool

	GetMusicBrowserRoot() string
}

type MusicDocWalker interface {
	WalkMusicDir(parentPath string) ([]SongDocument, error)
	Close()
}

type PlayQueue interface {
	GetPlaylist() ([]PlaylistItem, error)
	ClearPlaylist() error
	SavePlaylist(playlist []PlaylistItem) error
	Close()
}

type PlayerView interface {
	Refresh()
	Close()

	RefreshView() PlayerView
}

type Attrs map[string]string
