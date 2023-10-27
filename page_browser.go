// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/wildeyedskies/stmp/logger"
	"github.com/wildeyedskies/stmp/subsonic"
)

type BrowserPage struct {
	Root *tview.Flex

	artistList  *tview.List
	entityList  *tview.List
	searchField *tview.InputField

	currentDirectory *subsonic.SubsonicDirectory
	artistIdList     []string

	// external refs
	ui     *Ui
	logger logger.LoggerInterface
}

func (ui *Ui) createBrowserPage(indexes *[]subsonic.SubsonicIndex) (*tview.Flex, tview.Primitive) {
	browserPage := BrowserPage{
		ui:     ui,
		logger: ui.logger,

		currentDirectory: &subsonic.SubsonicDirectory{},
		artistIdList:     []string{},
	}

	// artist list
	browserPage.artistList = tview.NewList().
		ShowSecondaryText(false)
	browserPage.artistList.Box.
		SetTitle(" artist ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	for _, index := range *indexes {
		for _, artist := range index.Artists {
			browserPage.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
			browserPage.artistIdList = append(browserPage.artistIdList, artist.Id)
		}
	}

	// album list
	browserPage.entityList = tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true)
	browserPage.entityList.Box.
		SetTitle(" album ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// search bar
	browserPage.searchField = tview.NewInputField().
		SetLabel("search:").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetChangedFunc(func(s string) {
			idxs := browserPage.artistList.FindItems(s, "", false, true)
			if len(idxs) == 0 {
				return
			}
			browserPage.artistList.SetCurrentItem(idxs[0])
		}).
		SetDoneFunc(func(key tcell.Key) {
			ui.app.SetFocus(browserPage.artistList)
		})

	artistFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(browserPage.artistList, 0, 1, true).
		AddItem(browserPage.entityList, 0, 1, false)

	browserFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(artistFlex, 0, 1, true).
		AddItem(browserPage.searchField, 1, 0, false)

	// going right from the artist list should focus the album/song list
	browserPage.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(browserPage.entityList)
			return nil
		}
		switch event.Rune() {
		case '/':
			browserPage.search()
			return nil
		case 'n':
			browserPage.searchNext()
			return nil
		case 'N':
			browserPage.searchPrev()
			return nil
		case 'R':
			goBackTo := browserPage.artistList.GetCurrentItem()
			// REFRESH artists
			indexResponse, err := ui.connection.GetIndexes()
			if err != nil {
				ui.logger.Printf("Error fetching indexes from server: %s\n", err)
				return event
			}
			browserPage.artistList.Clear()
			ui.connection.ClearCache()
			for _, index := range indexResponse.Indexes.Index {
				for _, artist := range index.Artists {
					browserPage.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
					browserPage.artistIdList = append(browserPage.artistIdList, artist.Id)
				}
			}
			// Try to put the user to about where they were
			if goBackTo < browserPage.artistList.GetItemCount() {
				browserPage.artistList.SetCurrentItem(goBackTo)
			}
		}
		return event
	})

	browserPage.artistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		browserPage.handleEntitySelected(browserPage.artistIdList[index])
	})

	// "add to playlist" modal
	for _, playlist := range ui.playlists {
		ui.addToPlaylistList.AddItem(tview.Escape(playlist.Name), "", 0, nil)
	}
	ui.addToPlaylistList.SetBorder(true).
		SetTitle("Add to Playlist")

	addToPlaylistFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.addToPlaylistList, 0, 1, true)

	addToPlaylistModal := makeModal(addToPlaylistFlex, 60, 20)

	ui.addToPlaylistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.pages.HidePage("addToPlaylist")
			ui.pages.SwitchToPage("browser")
			ui.app.SetFocus(browserPage.entityList)
		} else if event.Key() == tcell.KeyEnter {
			playlist := ui.playlists[ui.addToPlaylistList.GetCurrentItem()]
			browserPage.handleAddSongToPlaylist(&playlist)

			ui.pages.HidePage("addToPlaylist")
			ui.pages.SwitchToPage("browser")
			ui.app.SetFocus(browserPage.entityList)
		}
		return event
	})

	browserPage.entityList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyLeft {
			ui.app.SetFocus(browserPage.artistList)
			return nil
		}
		if event.Rune() == 'a' {
			browserPage.handleAddEntityToQueue()
			return nil
		}
		if event.Rune() == 'y' {
			browserPage.handleToggleEntityStar()
			return nil
		}
		// only makes sense to add to a playlist if there are playlists
		if event.Rune() == 'A' {
			if ui.playlistList.GetItemCount() > 0 {
				ui.pages.ShowPage("addToPlaylist")
				ui.app.SetFocus(ui.addToPlaylistList)
			} else {
				ui.showMessageBox("No playlists available. Create one first.")
			}
			return nil
		}
		// REFRESH only the artist
		if event.Rune() == 'r' {
			artistIdx := browserPage.artistList.GetCurrentItem()
			entity := browserPage.artistIdList[artistIdx]
			//ui.logger.Printf("refreshing artist idx %d, entity %s (%s)", artistIdx, entity, ui.connection.directoryCache[entity].Directory.Name)
			ui.connection.RemoveCacheEntry(entity)
			browserPage.handleEntitySelected(browserPage.artistIdList[artistIdx])
			return nil
		}
		return event
	})

	return browserFlex, addToPlaylistModal
}

func (b *BrowserPage) IsSearchFocused(focused tview.Primitive) bool {
	return focused == b.searchField
}

func (b *BrowserPage) UpdateStars() {
	if b.currentDirectory != nil {
		b.handleEntitySelected(b.currentDirectory.Id)
	}
}

func (b *BrowserPage) handleAddEntityToQueue() {
	currentIndex := b.entityList.GetCurrentItem()
	if currentIndex < 0 {
		return
	}

	if currentIndex+1 < b.entityList.GetItemCount() {
		b.entityList.SetCurrentItem(currentIndex + 1)
	}

	// if we have a parent directory subtract 1 to account for the [..]
	// which would be index 0 in that case with index 1 being the first entity
	if b.currentDirectory.Parent != "" {
		currentIndex--
	}

	if currentIndex == -1 || len(b.currentDirectory.Entities) <= currentIndex {
		return
	}

	entity := b.currentDirectory.Entities[currentIndex]

	if entity.IsDirectory {
		b.addDirectoryToQueue(&entity)
	} else {
		b.ui.addSongToQueue(&entity)
	}

	b.ui.queuePage.UpdateQueue()
}

func (b *BrowserPage) handleEntitySelected(directoryId string) {
	response, err := b.ui.connection.GetMusicDirectory(directoryId)
	if err != nil {
		b.logger.Printf("handleEntitySelected: GetMusicDirectory %s -- %s", directoryId, err.Error())
	}
	sort.Sort(response.Directory.Entities)

	b.currentDirectory = &response.Directory
	b.entityList.Clear()
	if response.Directory.Parent != "" {
		// has parent entity
		b.entityList.Box.SetTitle(" song ")
		b.entityList.AddItem(tview.Escape("[..]"), "", 0,
			b.makeEntityHandler(response.Directory.Parent))
	} else {
		// no parent
		b.entityList.Box.SetTitle(" album ")
	}

	for _, entity := range response.Directory.Entities {
		var title string
		var id = entity.Id
		var handler func()
		if entity.IsDirectory {
			title = tview.Escape("[" + entity.Title + "]")
			handler = b.makeEntityHandler(entity.Id)
		} else {
			title = entityListTextFormat(entity, b.ui.starIdList)
			handler = makeSongHandler(id, b.ui.connection.GetPlayUrl(&entity),
				title, stringOr(entity.Artist, response.Directory.Name),
				entity.Duration, b.ui)
		}

		b.entityList.AddItem(tview.Escape(title), "", 0, handler)
	}
}

func (b *BrowserPage) makeEntityHandler(directoryId string) func() {
	return func() {
		b.handleEntitySelected(directoryId)
	}
}

func (b *BrowserPage) handleToggleEntityStar() {
	currentIndex := b.entityList.GetCurrentItem()
	if currentIndex < 0 {
		return
	}

	var entity = b.currentDirectory.Entities[currentIndex-1]

	// If the song is already in the star list, remove it
	_, remove := b.ui.starIdList[entity.Id]

	if _, err := b.ui.connection.ToggleStar(entity.Id, b.ui.starIdList); err != nil {
		b.logger.PrintError("ToggleStar", err)
		return
	}

	if remove {
		delete(b.ui.starIdList, entity.Id)
	} else {
		b.ui.starIdList[entity.Id] = struct{}{}
	}

	var text = entityListTextFormat(entity, b.ui.starIdList)
	updateEntityListItem(b.entityList, currentIndex, text)

	b.ui.StarsWereUpdated()
}

func entityListTextFormat(queueItem subsonic.SubsonicEntity, starredItems map[string]struct{}) string {
	var star = ""
	_, hasStar := starredItems[queueItem.Id]
	if hasStar {
		star = " [red]♥"
	}
	return queueItem.Title + star
}

// Just update the text of a specific row
func updateEntityListItem(entityList *tview.List, id int, text string) {
	entityList.SetItemText(id, text, "")
}

func (b *BrowserPage) addDirectoryToQueue(entity *subsonic.SubsonicEntity) {
	response, err := b.ui.connection.GetMusicDirectory(entity.Id)
	if err != nil {
		b.logger.Printf("addDirectoryToQueue: GetMusicDirectory %s -- %s", entity.Id, err.Error())
		return
	}

	sort.Sort(response.Directory.Entities)
	for _, e := range response.Directory.Entities {
		if e.IsDirectory {
			b.addDirectoryToQueue(&e)
		} else {
			// TODO maybe BrowserPage gets its own version of this function that uses dirname as artist name as fallback
			b.ui.addSongToQueue(&e)
		}
	}
}

func (b *BrowserPage) search() {
	name, _ := b.ui.pages.GetFrontPage()
	if name != "browser" {
		return
	}
	b.searchField.SetText("")
	b.ui.app.SetFocus(b.searchField)
}

func (b *BrowserPage) searchNext() {
	str := b.searchField.GetText()
	idxs := b.artistList.FindItems(str, "", false, true)
	if len(idxs) == 0 {
		return
	}

	curIdx := b.artistList.GetCurrentItem()
	for _, nidx := range idxs {
		if nidx > curIdx {
			b.artistList.SetCurrentItem(nidx)
			return
		}
	}
	b.artistList.SetCurrentItem(idxs[0])
}

func (b *BrowserPage) searchPrev() {
	str := b.searchField.GetText()
	idxs := b.artistList.FindItems(str, "", false, true)
	if len(idxs) == 0 {
		return
	}

	curIdx := b.artistList.GetCurrentItem()
	for nidx := len(idxs) - 1; nidx >= 0; nidx-- {
		if idxs[nidx] < curIdx {
			b.artistList.SetCurrentItem(idxs[nidx])
			return
		}
	}
	b.artistList.SetCurrentItem(idxs[len(idxs)-1])
}

func (b *BrowserPage) handleAddSongToPlaylist(playlist *subsonic.SubsonicPlaylist) {
	currentIndex := b.entityList.GetCurrentItem()

	// if we have a parent directory subtract 1 to account for the [..]
	// which would be index 0 in that case with index 1 being the first entity
	if b.currentDirectory.Parent != "" {
		currentIndex--
	}

	if currentIndex < 0 || len(b.currentDirectory.Entities) < currentIndex {
		return
	}

	entity := b.currentDirectory.Entities[currentIndex]

	if !entity.IsDirectory {
		if err := b.ui.connection.AddSongToPlaylist(string(playlist.Id), entity.Id); err != nil {
			b.logger.PrintError("AddSongToPlaylist", err)
			return
		}
	}

	// update the playlists
	response, err := b.ui.connection.GetPlaylists()
	if err != nil {
		b.logger.PrintError("GetPlaylists", err)
	}
	b.ui.playlists = response.Playlists.Playlists

	b.ui.playlistList.Clear()
	b.ui.addToPlaylistList.Clear()

	for _, playlist := range b.ui.playlists {
		b.ui.playlistList.AddItem(playlist.Name, "", 0, nil)
		b.ui.addToPlaylistList.AddItem(playlist.Name, "", 0, nil)
	}

	if currentIndex+1 < b.entityList.GetItemCount() {
		b.entityList.SetCurrentItem(currentIndex + 1)
	}
}
