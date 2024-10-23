// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/subsonic"
)

type SearchPage struct {
	Root               *tview.Flex
	AddToPlaylistModal tview.Primitive

	columnsFlex *tview.Flex

	artistList  *tview.List
	albumList   *tview.List
	songList    *tview.List
	searchField *tview.InputField

	artists []*subsonic.Artist
	albums  []*subsonic.Album
	songs   []*subsonic.SubsonicEntity

	// external refs
	ui     *Ui
	logger logger.LoggerInterface
}

func (ui *Ui) createSearchPage() *SearchPage {
	searchPage := SearchPage{
		ui:     ui,
		logger: ui.logger,
	}

	// artist list
	searchPage.artistList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.artistList.Box.
		SetTitle(" artist matches ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// album list
	searchPage.albumList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.albumList.Box.
		SetTitle(" album matches ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// song list
	searchPage.songList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.songList.Box.
		SetTitle(" song matches ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// search bar
	searchPage.searchField = tview.NewInputField().
		SetLabel("search:").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetDoneFunc(func(key tcell.Key) {
			searchPage.aproposFocus()
		})

	searchPage.columnsFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(searchPage.artistList, 0, 1, true).
		AddItem(searchPage.albumList, 0, 1, false).
		AddItem(searchPage.songList, 0, 1, false)

	searchPage.Root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(searchPage.columnsFlex, 0, 1, true).
		AddItem(searchPage.searchField, 1, 1, false)

	searchPage.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			ui.app.SetFocus(searchPage.songList)
			return nil
		case tcell.KeyRight:
			ui.app.SetFocus(searchPage.albumList)
			return nil
		case tcell.KeyEnter:
			idx := searchPage.artistList.GetCurrentItem()
			searchPage.addArtistToQueue(searchPage.artists[idx])
			return nil
		}

		switch event.Rune() {
		case 'a':
			idx := searchPage.artistList.GetCurrentItem()
			searchPage.logger.Printf("artistList adding (%d) %s", idx, searchPage.artists[idx].Name)
			searchPage.addArtistToQueue(searchPage.artists[idx])
			return nil
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		}

		return event
	})
	searchPage.albumList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			ui.app.SetFocus(searchPage.artistList)
			return nil
		case tcell.KeyRight:
			ui.app.SetFocus(searchPage.songList)
			return nil
		case tcell.KeyEnter:
			idx := searchPage.albumList.GetCurrentItem()
			searchPage.addAlbumToQueue(searchPage.albums[idx])
			return nil
		}

		switch event.Rune() {
		case 'a':
			idx := searchPage.albumList.GetCurrentItem()
			searchPage.logger.Printf("albumList adding (%d) %s", idx, searchPage.albums[idx].Name)
			searchPage.addAlbumToQueue(searchPage.albums[idx])
			return nil
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		}

		return event
	})
	searchPage.songList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			ui.app.SetFocus(searchPage.albumList)
			return nil
		case tcell.KeyRight:
			ui.app.SetFocus(searchPage.artistList)
			return nil
		case tcell.KeyEnter:
			idx := searchPage.songList.GetCurrentItem()
			ui.addSongToQueue(searchPage.songs[idx])
			ui.queuePage.UpdateQueue()
			return nil
		}

		switch event.Rune() {
		case 'a':
			idx := searchPage.songList.GetCurrentItem()
			ui.addSongToQueue(searchPage.songs[idx])
			ui.queuePage.updateQueue()
			return nil
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		}

		return event
	})
	search := make(chan string, 5)
	searchPage.searchField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyESC:
			searchPage.aproposFocus()
		case tcell.KeyEnter:
			search <- ""
			searchPage.artistList.Clear()
			searchPage.artists = make([]*subsonic.Artist, 0)
			searchPage.albumList.Clear()
			searchPage.albums = make([]*subsonic.Album, 0)
			searchPage.songList.Clear()
			searchPage.songs = make([]*subsonic.SubsonicEntity, 0)

			queryStr := searchPage.searchField.GetText()
			search <- queryStr
		default:
			return event
		}
		return nil
	})
	go searchPage.search(search)

	return &searchPage
}

func (s *SearchPage) search(search chan string) {
	var query string
	var artOff, albOff, songOff int
	more := make(chan bool, 5)
	for {
		// quit searching if we receive an interrupt
		select {
		case query = <-search:
			artOff = 0
			albOff = 0
			songOff = 0
			s.logger.Printf("searching for %q [%d, %d, %d]", query, artOff, albOff, songOff)
			for len(more) > 0 {
				<-more
			}
			if query == "" {
				continue
			}
		case <-more:
			s.logger.Printf("fetching more %q [%d, %d, %d]", query, artOff, albOff, songOff)
		}
		res, err := s.ui.connection.Search(query, artOff, albOff, songOff)
		if err != nil {
			s.logger.PrintError("SearchPage.search", err)
			return
		}
		// Quit searching if there are no more results
		if len(res.SearchResults.Artist) == 0 &&
			len(res.SearchResults.Album) == 0 &&
			len(res.SearchResults.Song) == 0 {
			continue
		}

		query = strings.ToLower(query)
		s.ui.app.QueueUpdate(func() {
			for _, artist := range res.SearchResults.Artist {
				if strings.Contains(strings.ToLower(artist.Name), query) {
					s.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
					s.artists = append(s.artists, &artist)
				}
			}
			s.artistList.Box.SetTitle(fmt.Sprintf(" artist matches (%d) ", len(s.artists)))
			for _, album := range res.SearchResults.Album {
				if strings.Contains(strings.ToLower(album.Name), query) {
					s.albumList.AddItem(tview.Escape(album.Name), "", 0, nil)
					s.albums = append(s.albums, &album)
				}
			}
			s.albumList.Box.SetTitle(fmt.Sprintf(" album matches (%d) ", len(s.albums)))
			for _, song := range res.SearchResults.Song {
				if strings.Contains(strings.ToLower(song.Title), query) {
					s.songList.AddItem(tview.Escape(song.Title), "", 0, nil)
					s.songs = append(s.songs, &song)
				}
			}
			s.songList.Box.SetTitle(fmt.Sprintf(" song matches (%d) ", len(s.songs)))
		})

		artOff += len(res.SearchResults.Artist)
		albOff += len(res.SearchResults.Album)
		songOff += len(res.SearchResults.Song)
		more <- true
	}
}

func (s *SearchPage) addArtistToQueue(entity subsonic.Ider) {
	response, err := s.ui.connection.GetArtist(entity.ID())
	if err != nil {
		s.logger.Printf("addArtistToQueue: GetArtist %s -- %s", entity.ID(), err.Error())
		return
	}

	artistId := response.Artist.Id
	for _, album := range response.Artist.Album {
		response, err = s.ui.connection.GetAlbum(album.Id)
		if err != nil {
			s.logger.Printf("error getting album %s while adding artist to queue", album.Id)
			return
		}
		sort.Sort(response.Album.Song)
		// We make sure we add only albums who's artists match the artist
		// being added; this prevents collection albums with many different
		// artists that show up in the Album column having _all_ of the songs
		// on the album -- even ones that don't match the artist -- from
		// being added when the user adds an album from the search results.
		for _, e := range response.Album.Song {
			// Depending on the server implementation, the server may or may not
			// respond with a list of artists. If either the Artist field matches,
			// or the artist name is in a list of artists, then we add the song.
			if e.ArtistId == artistId {
				s.ui.addSongToQueue(&e)
				continue
			}
			for _, art := range e.Artists {
				if art.Id == artistId {
					s.ui.addSongToQueue(&e)
					break
				}
			}
		}
	}

	s.ui.queuePage.UpdateQueue()
}

func (s *SearchPage) addAlbumToQueue(entity subsonic.Ider) {
	response, err := s.ui.connection.GetAlbum(entity.ID())
	if err != nil {
		s.logger.Printf("addToQueue: GetMusicDirectory %s -- %s", entity.ID(), err.Error())
		return
	}
	sort.Sort(response.Album.Song)
	for _, e := range response.Album.Song {
		s.ui.addSongToQueue(&e)
	}
	s.ui.queuePage.UpdateQueue()
}

func (s *SearchPage) aproposFocus() {
	if len(s.artists) != 0 {
		s.ui.app.SetFocus(s.artistList)
	} else if len(s.albums) != 0 {
		s.ui.app.SetFocus(s.albumList)
	} else if len(s.songs) != 0 {
		s.ui.app.SetFocus(s.songList)
	} else {
		s.ui.app.SetFocus(s.artistList)
	}
}
