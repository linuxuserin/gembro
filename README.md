# Gembro

A mouse-driven CLI Gemini client with Gopher support

Made with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework

## Features

- Browse Gemspace
- Mouse driven
- Up to 9 tabs!
- Bookmarks
- Download pages

## Keyboard

Some keys might not work in some terminals.

|Action|Key|
|--|--|
|Go back|h|
|Go forward|l|
|Open link|type number + enter|
|Open link in tab|type number + t|
|Quit|ctrl+c|
|Next tab|tab|
|Previous tab|shift+tab|
|Goto tab|alt+#|
|Close tab|q|
|Goto URL|g|
|Download page|d|
|Home|H|
|Bookmark|b|

## Mouse

Some buttons might not work in some terminals.

|Action|Button|
|--|--|
|Open link|Left click|
|Open link in tab|Middle click|
|Close tab|Middle click (on tab)|
|Go back|Right click|

## What's Gemini?

'You may think of Gemini as "the web, stripped right back to its essence" or as "Gopher, souped up and modernised just a little", depending upon your perspective (the latter view is probably more accurate).'

See [Project Gemini](https://gemini.circumlunar.space/)

## Run

```bash
go build
./gembro
```

## Todo

- Help screen
- Turn button bar in clickable buttons
- Fix download name and extension (consider media type)
