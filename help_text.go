// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

const helpPlayback = `
p      play/pause
P      stop
>      next song
-/=(+) volume down/volume up
,/.    seek -10/+10 seconds
r      add 50 random songs to queue
s      start server library scan
`

const helpPageBrowser = `
artist tab
  R     refresh the list
  /     Search artists
  a     Add all artist songs to queue
  n     Continue search forward
  N     Continue search backwards
song tab
  ENTER play song (clears current queue)
  a     add album or song to queue
  A     add song to playlist
  y     toggle star on song/album
  R     refresh the list
ESC   Close search
`

const helpPageQueue = `
d/DEL remove currently selected song from the queue
D     remove all songs from queue
y     toggle star on song
k     move selected song up in queue
j     move selected song down in queue
s     save queue as a playlist
S     shuffle the current queue
`

const helpPagePlaylists = `
n     new playlist
d     delete playlist
a     add playlist or song to queue
`

const helpSearchPage = `
artist, album/genre, or song tab
  Down    focus search field
  Left    previous column
  Right   next column
  /       start search
  a       recursively add item to quue
  g       toggle genre search
In album tab
  Enter   recursively add item to quue
In genre tab
  Enter   shows songs with genre
search field
  Enter   search for text
`
