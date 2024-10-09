// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/subsonic"
)

type BrowserPage struct {
	Root               *tview.Flex
	AddToPlaylistModal tview.Primitive

	artistFlex *tview.Flex

	artistList  *tview.List
	entityList  *tview.List
	searchField *tview.InputField

	currentDirectory *subsonic.SubsonicDirectory
	artistIdList     []string

	// external refs
	ui     *Ui
	logger logger.LoggerInterface
}

func (ui *Ui) createBrowserPage(indexes *[]subsonic.SubsonicIndex) *BrowserPage {
	browserPage := BrowserPage{
		ui:     ui,
		logger: ui.logger,

		currentDirectory: nil,
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

	browserPage.artistFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(browserPage.artistList, 0, 1, true).
		AddItem(browserPage.entityList, 0, 1, false)

	browserPage.Root = tview.NewFlex().SetDirection(tview.FlexRow)
	browserPage.showSearchField(false) // add artist/search items

	// going right from the artist list should focus the album/song list
	browserPage.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(browserPage.entityList)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			browserPage.showSearchField(false)
			ui.app.SetFocus(browserPage.artistList)
			return nil
		}

		switch event.Rune() {
		case 'a':
			browserPage.handleAddArtistToQueue()
			return nil
		case '/':
			browserPage.showSearchField(true)
			browserPage.search()
			return nil
		case 'n':
			browserPage.showSearchField(true)
			browserPage.searchNext()
			return nil
		case 'N':
			browserPage.showSearchField(true)
			browserPage.searchPrev()
			return nil
		case 'S':
			browserPage.handleAddRandomSongs("similar")
		case 'R':
			goBackTo := browserPage.artistList.GetCurrentItem()
			// REFRESH artists
			indexResponse, err := ui.connection.GetIndexes()
			if err != nil {
				ui.logger.Printf("Error fetching indexes from server: %s\n", err)
				return event
			}

			browserPage.artistList.Clear()
			browserPage.artistIdList = []string{}
			ui.connection.ClearCache()

			// Sort the indexes before adding to the list
			for _, index := range indexResponse.Indexes.Index {
				sort.Slice(index.Artists, func(i, j int) bool {
					return index.Artists[i].Name < index.Artists[j].Name
				})
				for _, artist := range index.Artists {
					browserPage.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
					browserPage.artistIdList = append(browserPage.artistIdList, artist.Id)
				}
			}

			// Try to put the user to about where they were
			if goBackTo < browserPage.artistList.GetItemCount() {
				browserPage.artistList.SetCurrentItem(goBackTo)
			}
			return nil
		}
		return event
	})

	browserPage.artistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		if index < len(browserPage.artistIdList) {
			browserPage.handleEntitySelected(browserPage.artistIdList[index])
		}
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

	browserPage.AddToPlaylistModal = makeModal(addToPlaylistFlex, 60, 20)

	ui.addToPlaylistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.pages.HidePage(PageAddToPlaylist)
			ui.pages.SwitchToPage(PageBrowser)
			ui.app.SetFocus(browserPage.entityList)
			return nil
		} else if event.Key() == tcell.KeyEnter {
			playlist := ui.playlists[ui.addToPlaylistList.GetCurrentItem()]
			browserPage.handleAddSongToPlaylist(&playlist)

			ui.pages.HidePage(PageAddToPlaylist)
			ui.pages.SwitchToPage(PageBrowser)
			ui.app.SetFocus(browserPage.entityList)
			return nil
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
		if event.Rune() == 'A' {
			// only makes sense to add to a playlist if there are playlists
			if ui.playlistPage.GetCount() > 0 {
				ui.pages.ShowPage(PageAddToPlaylist)
				ui.app.SetFocus(ui.addToPlaylistList)
			} else {
				ui.showMessageBox("No playlists available. Create one first.")
			}
			return nil
		}
		// REFRESH only the artist
		if event.Rune() == 'R' {
			artistIdx := browserPage.artistList.GetCurrentItem()
			entity := browserPage.artistIdList[artistIdx]
			//ui.logger.Printf("refreshing artist idx %d, entity %s (%s)", artistIdx, entity, ui.connection.directoryCache[entity].Directory.Name)
			ui.connection.RemoveCacheEntry(entity)
			browserPage.handleEntitySelected(browserPage.artistIdList[artistIdx])
			return nil
		}
		if event.Rune() == 'S' {
			browserPage.handleAddRandomSongs("similar")
		}
		return event
	})

	// open first artist by default so we don't get stuck when there's only one artist
	if len(browserPage.artistIdList) > 0 {
		browserPage.handleEntitySelected(browserPage.artistIdList[0])
	}

	return &browserPage
}

func (b *BrowserPage) showSearchField(visible bool) {
	b.Root.Clear()
	b.Root.AddItem(b.artistFlex, 0, 1, true)

	if visible {
		b.Root.AddItem(b.searchField, 1, 0, false)
	}
}

func (b *BrowserPage) IsSearchFocused(focused tview.Primitive) bool {
	return focused == b.searchField
}

func (b *BrowserPage) UpdateStars() {
	// reload album/song list if one is open
	if b.currentDirectory != nil {
		b.handleEntitySelected(b.currentDirectory.Id)
	}
}

func (b *BrowserPage) handleAddArtistToQueue() {
	currentIndex := b.artistList.GetCurrentItem()
	if b.artistList.GetCurrentItem() < 0 {
		return
	}

	sort.Sort(b.currentDirectory.Entities)

	for _, entity := range b.currentDirectory.Entities {
		if entity.IsDirectory {
			b.addDirectoryToQueue(&entity)
		} else {
			b.ui.addSongToQueue(&entity)
		}
	}

	if currentIndex+1 < b.artistList.GetItemCount() {
		b.artistList.SetCurrentItem(currentIndex + 1)
	}

	b.ui.queuePage.UpdateQueue()
}

func (b *BrowserPage) handleAddRandomSongs(randomType string) {
	currentIndex := b.entityList.GetCurrentItem()
	if b.currentDirectory.Parent != "" {
		// account for [..] entry that we show, see handleEntitySelected()
		currentIndex--
	}
	if currentIndex < 0 {
		return
	}

	entity := b.currentDirectory.Entities[currentIndex]

	b.ui.addRandomSongsToQueue(entity.Id, randomType)
	b.ui.queuePage.UpdateQueue()
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
	if directoryId == "" {
		return
	}

	if response, err := b.ui.connection.GetMusicDirectory(directoryId); err != nil || response == nil {
		b.logger.Printf("handleEntitySelected: GetMusicDirectory %s -- %v", directoryId, err)
		return
	} else {
		b.currentDirectory = &response.Directory
		sort.Sort(response.Directory.Entities)
	}

	b.entityList.Clear()
	if b.currentDirectory.Parent != "" {
		// has parent entity
		b.entityList.Box.SetTitle(" song ")
		b.entityList.AddItem(
			tview.Escape("[..]"), "", 0,
			b.makeEntityHandler(b.currentDirectory.Parent))
	} else {
		// no parent
		b.entityList.Box.SetTitle(" album ")
	}

	for _, entity := range b.currentDirectory.Entities {
		var handler func()
		title := entityListTextFormat(entity, b.ui.starIdList) // handles escaping

		if entity.IsDirectory {
			// it's an album/directory
			handler = b.makeEntityHandler(entity.Id)
		} else {
			// it's a song
			handler = makeSongHandler(&entity, b.ui, b.currentDirectory.Name)
		}

		b.entityList.AddItem(title, "", 0, handler)
	}
}

func (b *BrowserPage) makeEntityHandler(directoryId string) func() {
	return func() {
		b.handleEntitySelected(directoryId)
	}
}

func (b *BrowserPage) handleToggleEntityStar() {
	currentIndex := b.entityList.GetCurrentItem()
	originalIndex := currentIndex
	if b.currentDirectory.Parent != "" {
		// account for [..] entry that we show, see handleEntitySelected()
		currentIndex--
	}
	if currentIndex < 0 {
		return
	}

	entity := b.currentDirectory.Entities[currentIndex]

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

	// update entity list entry
	text := entityListTextFormat(entity, b.ui.starIdList)
	b.entityList.SetItemText(originalIndex, text, "")

	b.ui.queuePage.UpdateQueue()
}

func entityListTextFormat(entity subsonic.SubsonicEntity, starredItems map[string]struct{}) string {
	title := entity.Title
	if entity.IsDirectory {
		title = "[" + title + "]"
	}

	star := ""
	_, hasStar := starredItems[entity.Id]
	if hasStar {
		star = " [red]♥"
	}
	return tview.Escape(title) + star
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

	b.ui.playlistPage.UpdatePlaylists()

	if currentIndex+1 < b.entityList.GetItemCount() {
		b.entityList.SetCurrentItem(currentIndex + 1)
	}
}
