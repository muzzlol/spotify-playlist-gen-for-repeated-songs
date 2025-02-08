package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/zmb3/spotify"
)

var (
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

	playlists, err := client.GetPlaylistsForUser(context.Background(), user.ID)
	if err != nil {
		log.Fatalf("could not get playlists: %v", err)
	}

	playlistExists := false
	for _, playlist := range playlists.Playlists {
		if playlist.Name == "Repeats" {
			playlistExists = true
			repeatsPlaylist := playlist //make this globally avalible?
			break
		}
	}

	if !playlistExists {
		playlist, err := client.CreatePlaylistForUser(context.Background(), user.ID, "Repeats", "Playlist for tracks on repeat", false, false)
		repeatsPlaylist := playlist
		if err != nil {
			log.Fatalf("could not create playlist: %v", err)
		}
		fmt.Printf("Created playlist: %v", playlist)
	}

	recentlyPlayed, err := client.PlayerRecentlyPlayedOpt(context.Background(), &spotify.RecentlyPlayedOptions{
		Limit: &validListenTimes,
	})
	if err != nil {
		log.Fatalf("could not get recently played: %v", err)
	}

}
