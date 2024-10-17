// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package remote

type ControlledPlayer interface {
	// Returns true if a seek is currently in progress.
	IsSeeking() (bool, error)
	IsPaused() (bool, error)
	IsPlaying() (bool, error)

	// Registers a callback which is invoked when the player transitions to the Paused state.
	OnPaused(cb func())

	// Registers a callback which is invoked when the player transitions to the Stopped state.
	OnStopped(cb func())

	// Registers a callback which is invoked when the player transitions to the Playing state.
	OnPlaying(cb func())

	// Registers a callback which is invoked whenever a seek event occurs.
	OnSeek(cb func())

	OnSongChange(cb func(track TrackInterface))

	GetTimePos() float64

	Play() error
	Pause() error
	Stop() error
	SeekAbsolute(int) error
	NextTrack() error
	PreviousTrack() error

	SetVolume(percentValue int) error
}

type TrackInterface interface {
	GetId() string
	GetArtist() string
	GetTitle() string
	GetDuration() int
	GetAlbumArtist() string
	GetAlbum() string
	GetTrackNumber() int
	GetDiscNumber() int

	// something like ID != ""
	IsValid() bool
}
