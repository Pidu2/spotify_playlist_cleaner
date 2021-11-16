# Spotify Playlist Cleaner
## Use case
- You "un-like" songs on Spotify but they stay part of the playlists you added them to
- This script shows you which these songs are ("not liked but in playlist X")

## Usage
```bash
./cleaner <username>
```

## General Info
- Uses the Go wrapper for Spotify API https://github.com/zmb3/spotify
  - The program is a blatant copy of their authentication.go example
- Absolutely quick and even dirtier
- Tons of improvements needed to be remotely useful (remediation, exclude playlists, let the script be written by someone who knows how to write go... )
