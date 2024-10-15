// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/spezifisch/stmps/commands"
	"github.com/spezifisch/stmps/mpvplayer"
	"github.com/spezifisch/stmps/subsonic"
	tviewcommand "github.com/spezifisch/tview-command"
)

// >>> Stuff that will be moved to t-c
type MyEvent struct {
	tviewcommand.Event
}

func (e *MyEvent) IsCommand(name string) bool {
	return e.Command == name
}

// <<< End of Stuff
func (ui *Ui) handlePageInput(event *tcell.EventKey) *tcell.EventKey {
	focused := ui.app.GetFocus()
	if ui.playlistPage.IsNewPlaylistInputFocused(focused) ||
		ui.browserPage.IsSearchFocused(focused) ||
		focused == ui.searchPage.searchField ||
		ui.selectPlaylistWidget.visible {
		return event
	}

	tcEvent := tviewcommand.FromEventKey(event, ui.keyConfig)
	activeContext := ui.keyContextStack.Current()

	if err := tcEvent.LookupCommand(activeContext); err == nil && tcEvent.IsBound {
		ctx := &commands.CommandContext{
			Logger:      ui.logger,
			CurrentPage: activeContext,
		}

		err := ui.commandRegistry.Execute(ctx, tcEvent.Command)
		if err != nil {
			ui.logger.PrintError("t-c command execution", err)
		}
		return nil
	}

	return event // Pass event back if no command was handled
}

func (ui *Ui) ShowPage(name string) {
	ui.pages.SwitchToPage(name)
	ui.menuWidget.SetActivePage(name)
}

func (ui *Ui) Quit() {
	// TODO savePlayQueue/getPlayQueue
	ui.player.Quit()
	ui.app.Stop()
}

func (ui *Ui) handleAddRandomSongs(Id string, randomType string) {
	ui.addRandomSongsToQueue(Id, randomType)
	ui.queuePage.UpdateQueue()
}

func (ui *Ui) addRandomSongsToQueue(Id string, randomType string) {
	response, err := ui.connection.GetRandomSongs(Id, randomType)
	if err != nil {
		ui.logger.Printf("addRandomSongsToQueue %s", err.Error())
	}
	switch randomType {
	case "random":
		for _, e := range response.RandomSongs.Song {
			ui.addSongToQueue(&e)
		}
	case "similar":
		for _, e := range response.SimilarSongs.Song {
			ui.addSongToQueue(&e)
		}
	}
}

// make sure to call ui.QueuePage.UpdateQueue() after this
func (ui *Ui) addSongToQueue(entity *subsonic.SubsonicEntity) {
	uri := ui.connection.GetPlayUrl(entity)

	response, err := ui.connection.GetAlbum(entity.Parent)
	album := ""
	if err != nil {
		ui.logger.PrintError("addSongToQueue", err)
	} else {
		switch {
		case response.Album.Name != "":
			album = response.Album.Name
		case response.Album.Title != "":
			album = response.Album.Title
		case response.Album.Album != "":
			album = response.Album.Album
		}
	}

	queueItem := &mpvplayer.QueueItem{
		Id:          entity.Id,
		Uri:         uri,
		Title:       entity.GetSongTitle(),
		Artist:      entity.Artist,
		Duration:    entity.Duration,
		Album:       album,
		TrackNumber: entity.Track,
		CoverArtId:  entity.CoverArtId,
		DiscNumber:  entity.DiscNumber,
	}
	ui.player.AddToQueue(queueItem)
}

func makeSongHandler(entity *subsonic.SubsonicEntity, ui *Ui, fallbackArtist string) func() {
	// make copy of values so this function can be used inside a loop iterating over entities
	id := entity.Id
	// TODO: Why aren't we doing all of this _inside_ the returned func?
	uri := ui.connection.GetPlayUrl(entity)
	title := entity.Title
	artist := stringOr(entity.Artist, fallbackArtist)
	duration := entity.Duration
	track := entity.Track
	coverArtId := entity.CoverArtId
	disc := entity.DiscNumber

	response, err := ui.connection.GetAlbum(entity.Parent)
	album := ""
	if err != nil {
		ui.logger.PrintError("makeSongHandler", err)
	} else {
		switch {
		case response.Album.Name != "":
			album = response.Album.Name
		case response.Album.Title != "":
			album = response.Album.Title
		case response.Album.Album != "":
			album = response.Album.Album
		}
	}

	return func() {
		if err := ui.player.PlayUri(id, uri, title, artist, album, duration, track, disc, coverArtId); err != nil {
			ui.logger.PrintError("SongHandler Play", err)
			return
		}
		ui.queuePage.UpdateQueue()
	}
}
