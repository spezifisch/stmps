// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package mpvplayer

import "github.com/spezifisch/stmps/remote"

type QueueItem struct {
	Id       string
	Uri      string
	Title    string
	Artist   string
	Duration int
}

var _ remote.TrackInterface = (*QueueItem)(nil)

func (q QueueItem) GetAlbumArtist() string {
	return q.Artist
}

func (q QueueItem) GetArtist() string {
	return q.Artist
}

func (q QueueItem) GetTitle() string {
	return q.Title
}

func (q QueueItem) GetDuration() int {
	return q.Duration
}

func (q QueueItem) IsValid() bool {
	return q.Id != ""
}

func (q QueueItem) GetId() string {
	return q.Id
}

func (q QueueItem) GetUri() string {
	return q.Uri
}

func (q QueueItem) GetAlbum() string {
	return ""
}

func (q QueueItem) GetTrackNumber() int {
	return 0
}
