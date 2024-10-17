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

	artistOffset int
	albumOffset  int
	songOffset   int

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
		SetTitle(" artist ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// album list
	searchPage.albumList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.albumList.Box.
		SetTitle(" album ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// song list
	searchPage.songList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.songList.Box.
		SetTitle(" song ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// search bar
	searchPage.searchField = tview.NewInputField().
		SetLabel("search:").
		SetFieldBackgroundColor(tcell.ColorBlack)

	searchPage.columnsFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(searchPage.artistList, 0, 1, false).
		AddItem(searchPage.albumList, 0, 1, false).
		AddItem(searchPage.songList, 0, 1, false)

	searchPage.Root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(searchPage.columnsFlex, 0, 1, false).
		AddItem(searchPage.searchField, 1, 1, true)

	searchPage.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			ui.app.SetFocus(searchPage.songList)
			return nil
		case tcell.KeyRight:
			ui.app.SetFocus(searchPage.albumList)
			return nil
		case tcell.KeyEnter:
			if len(searchPage.artists) != 0 {
				idx := searchPage.artistList.GetCurrentItem()
				searchPage.addArtistToQueue(searchPage.artists[idx])
				return nil
			}
			return event
		}

		switch event.Rune() {
		case 'a':
			if len(searchPage.artists) != 0 {
				idx := searchPage.artistList.GetCurrentItem()
				searchPage.logger.Printf("artistList adding (%d) %s", idx, searchPage.artists[idx].Name)
				searchPage.addArtistToQueue(searchPage.artists[idx])
				return nil
			}
			return event
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		case 'n':
			searchPage.search()
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
			if len(searchPage.albums) != 0 {
				idx := searchPage.albumList.GetCurrentItem()
				searchPage.addAlbumToQueue(searchPage.albums[idx])
				return nil
			}
			return event
		}

		switch event.Rune() {
		case 'a':
			if len(searchPage.albums) != 0 {
				idx := searchPage.albumList.GetCurrentItem()
				searchPage.logger.Printf("albumList adding (%d) %s", idx, searchPage.albums[idx].Name)
				searchPage.addAlbumToQueue(searchPage.albums[idx])
				return nil
			}
			return event
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		case 'n':
			searchPage.search()
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
			if len(searchPage.artists) != 0 {
				idx := searchPage.songList.GetCurrentItem()
				ui.addSongToQueue(searchPage.songs[idx])
				ui.queuePage.UpdateQueue()
				return nil
			}
			return event
		}

		switch event.Rune() {
		case 'a':
			if len(searchPage.artists) != 0 {
				idx := searchPage.songList.GetCurrentItem()
				ui.addSongToQueue(searchPage.songs[idx])
				ui.queuePage.updateQueue()
				return nil
			}
			return event
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		case 'n':
			searchPage.search()
			return nil
		}

		return event
	})
	searchPage.searchField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyESC:
			if len(searchPage.artists) != 0 {
				ui.app.SetFocus(searchPage.artistList)
			} else if len(searchPage.albums) != 0 {
				ui.app.SetFocus(searchPage.albumList)
			} else if len(searchPage.songs) != 0 {
				ui.app.SetFocus(searchPage.songList)
			} else {
				ui.app.SetFocus(searchPage.artistList)
			}
		case tcell.KeyEnter:
			searchPage.artistList.Clear()
			searchPage.artists = make([]*subsonic.Artist, 0)
			searchPage.albumList.Clear()
			searchPage.albums = make([]*subsonic.Album, 0)
			searchPage.songList.Clear()
			searchPage.songs = make([]*subsonic.SubsonicEntity, 0)

			searchPage.artistOffset = 0
			searchPage.albumOffset = 0
			searchPage.songOffset = 0
			searchPage.search()
			if len(searchPage.artists) > 0 {
				ui.app.SetFocus(searchPage.artistList)
			} else if len(searchPage.albums) > 0 {
				ui.app.SetFocus(searchPage.albumList)
			} else if len(searchPage.songs) > 0 {
				ui.app.SetFocus(searchPage.songList)
			}
		default:
			return event
		}
		return nil
	})

	return &searchPage
}

func (s *SearchPage) search() {
	if len(s.searchField.GetText()) == 0 {
		return
	}
	query := s.searchField.GetText()

	res, err := s.ui.connection.Search(query, s.artistOffset, s.albumOffset, s.songOffset)
	if err != nil {
		s.logger.PrintError("SearchPage.search", err)
		return
	}

	for _, artist := range res.SearchResults.Artist {
		s.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
		s.artists = append(s.artists, &artist)
	}
	for _, album := range res.SearchResults.Album {
		s.albumList.AddItem(tview.Escape(album.Name), "", 0, nil)
		s.albums = append(s.albums, &album)
	}
	for _, song := range res.SearchResults.Song {
		s.songList.AddItem(tview.Escape(song.Title), "", 0, nil)
		s.songs = append(s.songs, &song)
	}

	s.artistOffset += len(res.SearchResults.Artist)
	s.albumOffset += len(res.SearchResults.Album)
	s.songOffset += len(res.SearchResults.Song)
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
