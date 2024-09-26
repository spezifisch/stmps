// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"text/template"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/mpvplayer"
)

// columns: star, title, artist, duration
const queueDataColumns = 4
const starIcon = "♥"

// data for rendering queue table
type queueData struct {
	tview.TableContentReadOnly

	// our copy of the queue
	playerQueue mpvplayer.PlayerQueue
	// we also need to know which elements are starred
	starIdList map[string]struct{}
}

var _ tview.TableContent = (*queueData)(nil)

type QueuePage struct {
	Root *tview.Flex

	queueList *tview.Table
	queueData queueData

	songInfo *tview.TextView

	// external refs
	ui     *Ui
	logger logger.LoggerInterface

	songInfoTemplate *template.Template
}

func (ui *Ui) createQueuePage() *QueuePage {
	tmpl := template.New("song info").Funcs(template.FuncMap{
		"formatTime": func(i int) string {
			return fmt.Sprintf("%s", time.Duration(i)*time.Second)
		},
	})
	songInfoTemplate, err := tmpl.Parse(songInfoTemplateString)
	if err != nil {
		ui.logger.PrintError("createQueuePage", err)
	}
	queuePage := QueuePage{
		ui:               ui,
		logger:           ui.logger,
		songInfoTemplate: songInfoTemplate,
	}

	// main table
	queuePage.queueList = tview.NewTable().
		SetSelectable(true, false). // rows selectable
		SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack))
	queuePage.queueList.Box.
		SetTitle(" queue ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)
	queuePage.queueList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDelete || event.Rune() == 'd' {
			queuePage.handleDeleteFromQueue()
		} else {
			switch event.Rune() {
			case 'y':
				queuePage.handleToggleStar()
			case 'j':
				queuePage.moveSongDown()
			case 'k':
				queuePage.moveSongUp()
			case 'S':
				queuePage.shuffle()
			default:
				return event
			}
		}

		return nil
	})

	// Song info
	queuePage.songInfo = tview.NewTextView()
	queuePage.songInfo.SetDynamicColors(true).SetScrollable(true).SetBorder(true).SetTitle("Song Info")

	queuePage.queueList.SetSelectionChangedFunc(queuePage.changeSelection)

	// flex wrapper
	queuePage.Root = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(queuePage.queueList, 0, 2, true).
		AddItem(queuePage.songInfo, 0, 1, false)

	// private data
	queuePage.queueData = queueData{
		starIdList: ui.starIdList,
	}

	return &queuePage
}

func (q *QueuePage) changeSelection(row, column int) {
	q.songInfo.Clear()
	if row >= len(q.queueData.playerQueue) || row < 0 || column < 0 {
		return
	}
	currentSong := q.queueData.playerQueue[row]
	_ = q.songInfoTemplate.Execute(q.songInfo, currentSong)
}

func (q *QueuePage) UpdateQueue() {
	q.updateQueue()
}

func (q *QueuePage) getSelectedItem() (index int, err error) {
	index, _ = q.queueList.GetSelection()
	if index < 0 {
		err = errors.New("invalid index")
		return
	}
	return
}

// button handler
func (q *QueuePage) handleDeleteFromQueue() {
	currentIndex, err := q.getSelectedItem()
	if err != nil {
		return
	}

	// remove the item from the queue
	q.ui.player.DeleteQueueItem(currentIndex)
	q.updateQueue()
}

// button handler
func (q *QueuePage) handleToggleStar() {
	starIdList := q.queueData.starIdList

	currentIndex, err := q.getSelectedItem()
	if err != nil {
		q.logger.PrintError("handleToggleStar", err)
		return
	}

	entity, err := q.ui.player.GetQueueItem(currentIndex)
	if err != nil {
		q.logger.PrintError("handleToggleStar", err)
		return
	}

	// If the song is already in the star list, remove it
	_, remove := starIdList[entity.Id]

	// update on server
	if _, err = q.ui.connection.ToggleStar(entity.Id, starIdList); err != nil {
		q.ui.showMessageBox("ToggleStar failed")
		return // fail, assume not toggled
	}

	if remove {
		delete(starIdList, entity.Id)
	} else {
		starIdList[entity.Id] = struct{}{}
	}

	q.ui.browserPage.UpdateStars()
}

// re-read queue data from mpvplayer which is the authoritative source for the queue
func (q *QueuePage) updateQueue() {
	queueWasEmpty := len(q.queueData.playerQueue) == 0

	// tell tview table to update its data
	q.queueData.playerQueue = q.ui.player.GetQueueCopy()
	q.queueList.SetContent(&q.queueData)

	// by default we're scrolled down after initially adding rows, fix this
	if queueWasEmpty {
		q.queueList.ScrollToBeginning()
	}

	r, c := q.queueList.GetSelection()
	q.changeSelection(r, c)
}

// moveSongUp moves the currently selected song up in the queue
// If the selected song isn't the third or higher, this is a NOP
// and no error is reported.
func (q *QueuePage) moveSongUp() {
	if len(q.queueData.playerQueue) == 0 {
		return
	}

	currentIndex, column := q.queueList.GetSelection()
	if currentIndex < 0 || column < 0 {
		q.logger.Printf("moveSongUp: invalid selection (%d, %d)", currentIndex, column)
		return
	}

	if currentIndex == 0 {
		return
	}

	if currentIndex == 1 {
		q.ui.player.Stop()
	}

	// remove the item from the queue
	q.ui.player.MoveSongUp(currentIndex)
	q.queueList.Select(currentIndex-1, column)
	q.updateQueue()
}

// moveSongUp moves the currently selected song up in the queue
// If the selected song is not the second-to-the-last or lower, this is a NOP,
// and no error is reported
func (q *QueuePage) moveSongDown() {
	queueLen := len(q.queueData.playerQueue)
	if queueLen == 0 {
		return
	}

	currentIndex, column := q.queueList.GetSelection()
	if currentIndex < 0 || column < 0 {
		q.logger.Printf("moveSongDown: invalid selection (%d, %d)", currentIndex, column)
		return
	}

	if currentIndex == 0 {
		q.ui.player.Stop()
	}

	if currentIndex > queueLen-2 {
		q.logger.Printf("moveSongDown: can't move last song")
		return
	}

	// remove the item from the queue
	q.ui.player.MoveSongDown(currentIndex)
	q.queueList.Select(currentIndex+1, column)
	q.updateQueue()
}

func (q *QueuePage) shuffle() {
	if len(q.queueData.playerQueue) == 0 {
		return
	}

	q.ui.player.Stop()
	q.ui.player.Shuffle()

	q.queueList.Select(0, 0)
	q.updateQueue()
}

// queueData methods, used by tview to lazily render the table
func (q *queueData) GetCell(row, column int) *tview.TableCell {
	if row >= len(q.playerQueue) || column >= queueDataColumns || row < 0 || column < 0 {
		return nil
	}
	song := q.playerQueue[row]

	switch column {
	case 0: // star
		text := " "
		color := tcell.ColorDefault
		if _, starred := q.starIdList[song.Id]; starred {
			text = starIcon
			color = tcell.ColorRed
		}
		return &tview.TableCell{
			Text:        text,
			Color:       color,
			Expansion:   0,
			MaxWidth:    1,
			Transparent: true,
		}
	case 1: // title
		return &tview.TableCell{
			Text:        tview.Escape(song.Title),
			Expansion:   1,
			Transparent: true,
		}
	case 2: // artist
		return &tview.TableCell{
			Text:        tview.Escape(song.Artist),
			Expansion:   1,
			Transparent: true,
		}
	case 3: // duration
		min, sec := iSecondsToMinAndSec(song.Duration)
		text := fmt.Sprintf("%3d:%02d", min, sec)
		return &tview.TableCell{
			Text:        text,
			Align:       tview.AlignRight,
			Expansion:   0,
			MaxWidth:    6,
			Transparent: true,
		}
	}

	return nil
}

// Return the total number of rows in the table.
func (q *queueData) GetRowCount() int {
	return len(q.playerQueue)
}

// Return the total number of columns in the table.
func (q *queueData) GetColumnCount() int {
	return queueDataColumns
}

var songInfoTemplateString = `[blue::b]Title:[-:-:-:-] [green::i]{{.Title}}[-:-:-:-]
[blue::b]Artist:[-:-:-:-] [::i]{{.Artist}}[-:-:-:-]
[blue::b]Album:[-:-:-:-] [::i]{{.GetAlbum}}[-:-:-:-]
[blue::b]Track:[-:-:-:-] [::i]{{.GetTrackNumber}}[-:-:-:-]
[blue::b]Duration:[-:-:-:-] [::i]{{formatTime .Duration}}[-:-:-:-] `
