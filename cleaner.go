package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"github.com/zmb3/spotify"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:8080/callback"

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadPrivate, spotify.ScopeUserLibraryRead)
	ch    = make(chan *spotify.Client)
	state = strconv.Itoa(int(rand.Int63()))
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Pass your username as Argument")
		os.Exit(1)
	}
	// first start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go http.ListenAndServe(":8080", nil)

	url := auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	client := <-ch

	// use the client to make calls that require authorization
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("You are logged in as:", user.ID)

	// used for getting data "piece by piece" (50 is limit on Spotify API)
	nbr := 50
	off := 0
	var all_tracks []spotify.SavedTrack = []spotify.SavedTrack{}
	fmt.Printf("Number of gathered liked tracks: ")
	// get tracks as long as there are more
	for {
		limitOpt := spotify.Options{Limit: &nbr, Offset: &off}
		// get all liked tracks of the user
		tracks, err := client.CurrentUsersTracksOpt(&limitOpt)
		if err != nil || len(tracks.Tracks) == 0 {
			break
		}
		all_tracks = append(all_tracks, tracks.Tracks...)
		off += 50
		fmt.Printf("%d...", len(all_tracks))
	}
	fmt.Println()
	// fill an array with only the IDs
	track_ids := []spotify.ID{}
	for _, element := range all_tracks {
		track_ids = append(track_ids, element.FullTrack.SimpleTrack.ID)
	}

	nbr = 50
	off = 0
	var all_playlists []spotify.SimplePlaylist = []spotify.SimplePlaylist{}
	fmt.Printf("Number of gathered playlists: ")
	// get playlists as long as there are more
	for {
		limitOpt := spotify.Options{Limit: &nbr, Offset: &off}
		playlists, err := client.GetPlaylistsForUserOpt(os.Args[1], &limitOpt)
		if err != nil || len(playlists.Playlists) == 0 {
			break
		}
		all_playlists = append(all_playlists, playlists.Playlists...)
		off += 50
		fmt.Printf("%d...", len(all_playlists))
	}
	fmt.Println()
	fmt.Println()

	// define empty map
	playlist_map := make(map[string][]spotify.PlaylistTrack)

	// go through all playlists..
	for _, element := range all_playlists {
		nbr = 50
		off = 0
		// and get all tracks of all the playlist.. as long as there are more
		for {
			limitOpt := spotify.Options{Limit: &nbr, Offset: &off}
			playlist_tracks, err := client.GetPlaylistTracksOpt(element.ID, &limitOpt, "")
			if err != nil || len(playlist_tracks.Tracks) == 0 {
				break
			}
			playlist_map[element.Name] = append(playlist_map[element.Name], playlist_tracks.Tracks...)
			off += 50
		}
	}

	// compare playlist track IDs with the liked ones
	fmt.Printf("%-20s%-40s%-20s\n", "PLAYLIST", "TRACK", "ARTISTS")
	for key, element := range playlist_map {
		for _, playlist_song := range element {
			inIDs := false
			for _, id := range track_ids {
				if id == playlist_song.Track.SimpleTrack.ID {
					inIDs = true
				}
			}
			if !inIDs && playlist_song.Track.PreviewURL != "" {
				artist_text := ""
				for _, artist := range playlist_song.Track.SimpleTrack.Artists {
					artist_text = artist_text + artist.Name + ", "
				}
				fmt.Printf("%-20s%-40s%-20s\n", key, playlist_song.Track.SimpleTrack.Name, artist_text)
			}
		}
	}
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}
	// use the token to get an authenticated client
	client := auth.NewClient(tok)
	fmt.Fprintf(w, "Login Completed!")
	ch <- &client
}
