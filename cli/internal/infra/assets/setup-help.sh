#!/bin/bash
# Writes a brief quick-reference of useful micro and zellij shortcuts to
# ~/help. Run as devuser at build time from the Base module so every container
# ships the cheat-sheet. View it any time with `cat ~/help` or `micro ~/help`.
set -e

cat > "$HOME/help" <<'HELP'
============================================================
 Devcontainer quick reference  ~  micro & zellij shortcuts
============================================================
View this file any time with:  cat ~/help   (or)   micro ~/help

------------------------------------------------------------
 micro  (terminal text editor)   ->   run:  micro <file>
------------------------------------------------------------
File
  Ctrl-S            Save
  Ctrl-Q            Quit (asks to save if there are changes)
  Ctrl-O            Open a file

Edit
  Ctrl-Z / Ctrl-Y   Undo / Redo
  Ctrl-C / Ctrl-X   Copy / Cut  (whole line when nothing selected)
  Ctrl-V            Paste
  Ctrl-K            Cut current line
  Ctrl-D            Duplicate current line
  Ctrl-A            Select all
  Tab / Shift-Tab   Indent / Un-indent selection
  Alt-Up / Alt-Down Move current line up / down
  Ctrl-/            Toggle comment on the line/selection

Navigate & search
  Ctrl-F            Find
  Ctrl-N / Ctrl-P   Find next / previous
  Ctrl-Left/Right   Jump word by word
  Ctrl-Home / End   Jump to start / end of file

Command bar & help
  Ctrl-E            Command mode, then type e.g.:
                      goto 42          jump to line 42
                      replace foo bar  find & replace
                      set tabsize 2    change a setting
                      vsplit <file>    open a vertical split
  Ctrl-G            Toggle the built-in keybindings help

------------------------------------------------------------
 zellij  (terminal multiplexer)  ->  run:  zellij   (or `zellij attach`)
------------------------------------------------------------
zellij is modal: press a Ctrl-<key> to enter a mode, then a
single letter. Press Enter or Esc to leave a mode.

Modes (press Ctrl-<key> first)
  Ctrl-p            Pane mode
  Ctrl-t            Tab mode
  Ctrl-n            Resize mode
  Ctrl-s            Scroll / search mode
  Ctrl-o            Session mode
  Ctrl-q            Quit zellij
  Ctrl-g            Lock / unlock (ignore all shortcuts)

Panes  (Ctrl-p then...)
  n                 New pane
  d / r             Split down / right
  x                 Close focused pane
  f                 Toggle fullscreen for the pane
  w                 Toggle floating panes
  arrows / h j k l  Move focus between panes

Tabs  (Ctrl-t then...)
  n                 New tab
  x                 Close tab
  r                 Rename tab
  arrows / h l      Previous / next tab
  1..9              Jump to tab number

Resize  (Ctrl-n then...)   arrows / h j k l to grow, + / - to fine-tune
Scroll  (Ctrl-s then...)   arrows / PgUp / PgDn ; press `e` to edit
                           the scrollback in micro, `s` to search

Session  (Ctrl-o then...)
  d                 Detach (leave it running; re-enter with `zellij attach`)
  w                 Open the session manager

Quick keys (no mode needed)
  Alt-n             New pane
  Alt-h/j/k/l       Move focus between panes
  Alt-+ / Alt--     Resize focused pane
  Alt-[ / Alt-]     Cycle layouts

============================================================
HELP

echo "Wrote quick reference to $HOME/help"
