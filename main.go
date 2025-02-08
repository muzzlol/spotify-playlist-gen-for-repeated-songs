package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/zmb3/spotify"
)

var (
	runTimer         int = 30
	validListenTimes int = 5
)

func main() {
	http.HandleFunc("/auth/spotify/login", spotifyLoginHandler)
	http.HandleFunc("/auth/spotify/callback", spotifyCallbackHandler)

	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	client := waitForAuthentication()
	fmt.Printf("Logged in as: %v", client)

	user, err := client.CurrentUser(context.Background())
	if err != nil {
		log.Fatalf("could not get user: %v", err)
	}
	var repeatsPlaylist *spotify.SimplePlaylist

	playlists, err := client.GetPlaylistsForUser(context.Background(), user.ID)
	if err != nil {
		log.Fatalf("could not get playlists: %v", err)
	}

	playlistExists := false
	for _, playlist := range playlists.Playlists {
		if playlist.Name == "Repeats" {
			playlistExists = true
			repeatsPlaylist = &playlist
			break
		}
	}

	if !playlistExists {
		playlist, err := client.CreatePlaylistForUser(context.Background(), user.ID, "Repeats", "Playlist for tracks on repeat", false)
		if err != nil {
			log.Fatalf("could not create playlist: %v", err)
		}
		repeatsPlaylist = &spotify.SimplePlaylist{
			Name: playlist.Name,
			ID:   playlist.ID,
		}
		fmt.Printf("Created playlist: %s\n", playlist.Name)
	}

	fmap := make(map[string]int)

	ticker := time.NewTicker(time.Duration(runTimer) * time.Minute)
	defer ticker.Stop()

	// function runs every time ticker ticks otherwise nothing happens between ticks
	for range ticker.C {

	}

}
