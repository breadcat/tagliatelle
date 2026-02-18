# Tagliatelle

>Previously called *[taggart](https://en.wikipedia.org/wiki/Taggart)*, renamed for unfortunate rhyming reasons

A simple golang application to provide a flexible SQLite tagging database and file browser via web browser.

Very rough around the edges, but functional. Primarily intended for personal use.

## Running

```
cd tagliatelle
go get github.com/mattn/go-sqlite3
go run .
```

Then access the server via a web browser, the default port is 8080.

## Features
* Multiple tags per category
* Bulk tag management via `file-id` or `tag:value` query
* Search through file names, descriptions or tag values with wildcard support
* Image, video, text and cbz gallery viewers
* Will transcode incompatible video formats
* Tag value aliases, e.g. `color:blue` and `color:navy`
* Regenerate video thumbnails via web interface
* Add files via local upload, remote upload or `yt-dlp` directly
* Clickable [rotate90](## "Rotates video/image contents by angle on click"), [l45](## "Jumps to line number in text viewer on click"), [01:23](## "Jumps video playback to specified timestamp on click") and [file/1234](## "Clickable link to that file ID") shortcodes in file descriptions
* Artbitrary searchable descriptions on files
* Raw file URI copying for external application access
* In browser file management (delete, rename)
* Self-organising, categorised notes, with optional `sed` operation rules
* Orphan and reverse orphan finding
* Database backup and vacuum support

## Limitations
* SQLite requires cgo, which requires gcc. Build/run with `CGO_ENABLED=1`
* Database deletions get reserved so you won't have sequential file ID's
* Paths are stored absolutely, not relatively, so moving your file store requires manual intervention

## Credits
* [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) for the go SQLite3 library
* [Fluent UI System Icons](https://icones.js.org/collection/fluent) for SVG icons used