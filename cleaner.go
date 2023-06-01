package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/zmb3/spotify"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const (
	redirectURI = "http://localhost:8080/callback"
	NUM_WORKER  = 20
)

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadPrivate, spotify.ScopeUserLibraryRead)
	ch    = make(chan *spotify.Client)
	state = strconv.Itoa(int(rand.Int63()))
)

func getLikedTracks(workerNumber int, client spotify.Client, results chan spotify.ID, wg *sync.WaitGroup) {
	defer wg.Done()
	// used for getting data "piece by piece" (50 is limit on Spotify API)
	nbr := 50
	for off := workerNumber * 50; off >= 0; off += NUM_WORKER * 50 {
		limitOpt := spotify.Options{Limit: &nbr, Offset: &off}
		tracks, err := client.CurrentUsersTracksOpt(&limitOpt)
		if err != nil || len(tracks.Tracks) == 0 {
			break
		}
		for _, track := range tracks.Tracks {
			results <- track.FullTrack.SimpleTrack.ID
		}
	}
}

func readLikedTracks(all_tracks *[]spotify.ID, results chan spotify.ID) {
	for trackId := range results {
		*all_tracks = append(*all_tracks, trackId)
	}
}

func getPlaylists(workerNumber int, client spotify.Client, results chan spotify.SimplePlaylist, wg *sync.WaitGroup) {
	defer wg.Done()
	// used for getting data "piece by piece" (50 is limit on Spotify API)
	nbr := 50
	for off := workerNumber * 50; off >= 0; off += NUM_WORKER * 50 {
		limitOpt := spotify.Options{Limit: &nbr, Offset: &off}
		playlists, err := client.GetPlaylistsForUserOpt(os.Args[1], &limitOpt)
		if err != nil || len(playlists.Playlists) == 0 {
			break
		}
		for _, playlist := range playlists.Playlists {
			results <- playlist
		}
	}
}

func readPlaylists(all_playlists *[]spotify.SimplePlaylist, results chan spotify.SimplePlaylist) {
	for playlist := range results {
		*all_playlists = append(*all_playlists, playlist)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Pass your username as Argument")
		os.Exit(1)
	}

	// AUTHENTICATION
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

	// TRACKS
	trackResults := make(chan spotify.ID)
	var wg sync.WaitGroup

	wg.Add(NUM_WORKER)
	for worker := 0; worker < NUM_WORKER; worker++ {
		go getLikedTracks(worker, *client, trackResults, &wg)
	}

	var all_tracks []spotify.ID
	go readLikedTracks(&all_tracks, trackResults)
	wg.Wait()
	close(trackResults)
	fmt.Println("Number of liked tracks: ", len(all_tracks))

	// PLAYLISTS
	playlistResults := make(chan spotify.SimplePlaylist)

	wg.Add(NUM_WORKER)
	for worker := 0; worker < NUM_WORKER; worker++ {
		go getPlaylists(worker, *client, playlistResults, &wg)
	}

	var all_playlists []spotify.SimplePlaylist
	go readPlaylists(&all_playlists, playlistResults)
	wg.Wait()
	close(playlistResults)
	fmt.Println("Number of playlists: ", len(all_playlists))

	fmt.Println()
	fmt.Println()

	// define empty map
	playlist_map := make(map[string][]spotify.PlaylistTrack)

	// go through all playlists..
	for _, element := range all_playlists {
		nbr := 50
		off := 0
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
			for _, id := range all_tracks {
				if id == playlist_song.Track.SimpleTrack.ID {
					inIDs = true
				}
			}
			if !inIDs && playlist_song.Track.SimpleTrack.ID != "" {
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
