package main

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type PlaylistSelectionWidget struct {
	Root             *tview.Flex
	ui               *Ui
	inputField       *tview.InputField
	overwrite        *tview.Checkbox
	accept           *tview.Button
	cancel           *tview.Button
	overwriteEnabled bool
	visible          bool
}

// createPlaylistSelectionWidget creates the widget and sets up all of the
// behaviors, including the key bindings.
func (ui *Ui) createPlaylistSelectionWidget() (m *PlaylistSelectionWidget) {
	m = &PlaylistSelectionWidget{
		ui: ui,
	}

	m.overwrite = tview.NewCheckbox()
	m.overwrite.SetDisabled(true)
	m.overwriteEnabled = false
	m.overwrite.SetLabel("Overwrite?").SetFieldTextColor(tcell.ColorBlack)
	m.overwrite.SetBackgroundColor(tcell.ColorGray)
	m.overwrite.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == ' ' {
			m.overwrite.SetChecked(!m.overwrite.IsChecked())
			m.accept.SetDisabled(!m.overwrite.IsChecked())
			return nil
		}
		return event
	})
	m.accept = tview.NewButton("Accept").SetLabelColor(tcell.ColorBlack)
	m.cancel = tview.NewButton("Cancel").SetLabelColor(tcell.ColorBlack)
	m.inputField = tview.NewInputField().SetAutocompleteFunc(func(current string) []string {
		rv := make([]string, 0)
		var exactMatch bool
		for _, p := range ui.playlists {
			if strings.Contains(p.Name, current) {
				rv = append(rv, p.Name)
			}
			if p.Name == current {
				exactMatch = true
			}
		}
		if exactMatch {
			m.overwrite.SetDisabled(false)
			m.overwriteEnabled = true
			m.accept.SetDisabled(!m.overwrite.IsChecked())
		} else {
			m.overwrite.SetDisabled(true)
			m.overwriteEnabled = false
			m.accept.SetDisabled(false)
		}
		return rv
	}).SetFieldTextColor(tcell.ColorBlack)
	m.inputField.SetDoneFunc(func(key tcell.Key) {
		m.focusNext(nil)
	})
	// FIXME with this code in place, the list isn't navigable
	// m.inputField.SetAutocompletedFunc(func(text string, index int, source int) bool {
	// 	m.inputField.SetText(text)
	// 	for _, p := range ui.playlists {
	// 		if p.Name == text {
	// 			m.overwrite.SetDisabled(false)
	// 			m.overwriteEnabled = true
	// 			m.focusNext(nil)
	// 			return false
	// 		}
	// 	}
	// 	m.overwrite.SetDisabled(true)
	// 	m.overwriteEnabled = false
	// 	return false
	// })
	acceptFunc := func() {
		inputText := m.inputField.GetText()
		if !m.overwrite.IsChecked() {
			for _, p := range ui.playlists {
				if p.Name == inputText {
					return
				}
			}
		}
		ui.queuePage.saveQueue(inputText)
		ui.CloseSelectPlaylist()
	}
	m.accept.SetSelectedFunc(acceptFunc)
	cancelFunc := func() {
		m.inputField.SetText("")
		m.overwrite.SetDisabled(true)
		m.overwriteEnabled = false
		m.overwrite.SetChecked(false)
		m.accept.SetDisabled(!m.overwrite.IsChecked())
		ui.CloseSelectPlaylist()
	}
	m.cancel.SetSelectedFunc(cancelFunc)

	buttons := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tview.NewFlex(), 0, 1, false).
		AddItem(m.accept, 0, 4, false).
		AddItem(tview.NewFlex(), 0, 1, false).
		AddItem(m.cancel, 0, 4, false).
		AddItem(tview.NewFlex(), 0, 1, false)

	m.Root = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(m.inputField, 1, 1, true).
		AddItem(m.overwrite, 1, 1, false).
		AddItem(buttons, 0, 1, false)

	m.Root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if m.ui.app.GetFocus() == m.inputField {
			switch event.Key() {
			case tcell.KeyTab:
				if event.Modifiers()&tcell.ModShift != 0 {
					return m.focusPrev(event)
				} else {
					return m.focusNext(event)
				}
			case tcell.KeyBacktab:
				return m.focusPrev(event)
			case tcell.KeyESC:
				cancelFunc()
			}
			return event
		}
		if event.Rune() == ' ' {
			focused := m.ui.app.GetFocus()
			if focused == m.accept {
				acceptFunc()
				return nil
			}
			if focused == m.cancel {
				cancelFunc()
				return nil
			}
			return event
		}
		switch event.Key() {
		case tcell.KeyESC:
			cancelFunc()
			return nil
		case tcell.KeyCR:
			focused := m.ui.app.GetFocus()
			if focused == m.accept {
				acceptFunc()
				return nil
			} else if focused == m.cancel {
				cancelFunc()
				return nil
			}
			m.focusNext(event)
			return event
		case tcell.KeyTab:
			if event.Modifiers()&tcell.ModShift != 0 {
				return m.focusPrev(event)
			} else {
				return m.focusNext(event)
			}
		case tcell.KeyBacktab:
			return m.focusPrev(event)
		case tcell.KeyDown:
			return m.focusNext(event)
		case tcell.KeyUp:
			return m.focusPrev(event)
		default:
			m.ui.logger.Printf("non-input key = %d", event.Key())
		}
		return event
	})

	m.Root.Box.SetBorder(true).SetTitle(" Playlist Name ")

	return
}

func (m *PlaylistSelectionWidget) focusNext(event *tcell.EventKey) *tcell.EventKey {
	switch m.ui.app.GetFocus() {
	case m.inputField:
		st := m.inputField.GetText()
		found := false
		for _, p := range m.ui.playlists {
			if p.Name == st {
				m.overwrite.SetDisabled(false)
				m.overwriteEnabled = true
				m.accept.SetDisabled(!m.overwrite.IsChecked())
				m.ui.app.SetFocus(m.overwrite)
				found = true
			}
		}
		if !found {
			m.overwrite.SetDisabled(true)
			m.overwriteEnabled = false
			m.accept.SetDisabled(false)
			m.ui.app.SetFocus(m.accept)
		}
	case m.overwrite:
		if m.overwrite.IsChecked() {
			m.ui.app.SetFocus(m.accept)
		} else {
			m.ui.app.SetFocus(m.cancel)
		}
	case m.accept:
		m.ui.app.SetFocus(m.cancel)
	case m.cancel:
		m.ui.app.SetFocus(m.inputField)
	default:
		return event
	}
	return nil
}

func (m PlaylistSelectionWidget) focusPrev(event *tcell.EventKey) *tcell.EventKey {
	switch m.ui.app.GetFocus() {
	case m.inputField:
		m.ui.app.SetFocus(m.cancel)
	case m.overwrite:
		m.ui.app.SetFocus(m.inputField)
	case m.accept:
		if m.overwriteEnabled {
			m.ui.app.SetFocus(m.overwrite)
		} else {
			m.ui.app.SetFocus(m.inputField)
		}
	case m.cancel:
		// FIXME There's some bug in back-tabbing from cancel; _something_ is disabling the overwriteEnabled field, and I can't find it. Tabbing forward works fine, but tabbing backward fails to work properly when the playlist name matches an existing playlist
		if m.overwriteEnabled {
			if m.overwrite.IsChecked() {
				m.ui.app.SetFocus(m.accept)
			} else {
				m.ui.app.SetFocus(m.overwrite)
			}
		} else {
			m.ui.app.SetFocus(m.accept)
		}
	default:
		return event
	}
	return nil
}
