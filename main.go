package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/zmb3/spotify/v2"
)

var (
	pollInterval     time.Duration = 1 * time.Minute // Duration in-between api calls
	validListenTimes int           = 5               // Track add if plays exceed
	fmapLimit        int           = 7               // Limit plays for songs
	decayThreshold   int           = 10              // API calls are decremented if return exceeds
	afterTime        int64                           // unix after value
)

func main() {
	http.HandleFunc("/auth/spotify/login", spotifyLoginHandler)
	http.HandleFunc("/auth/spotify/callback", spotifyCallbackHandler)

	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	client := waitForAuthentication()

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
		playlist, err := client.CreatePlaylistForUser(context.Background(), user.ID, "Repeats", "Playlist for tracks on repeat", false, false)
		if err != nil {
			log.Fatalf("could not create playlist: %v", err)
		}
		repeatsPlaylist = &spotify.SimplePlaylist{
			Name: playlist.Name,
			ID:   playlist.ID,
		}
		fmt.Printf("Created playlist: %s\n", playlist.Name)
	}

	fmap := make(map[spotify.ID]int) // Store fmap with track id as spotify.ID

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// function runs every time ticker ticks otherwise nothing happens between ticks
	for range ticker.C {
		recentlyPlayed, err := client.PlayerRecentlyPlayedOpt(context.Background(), &spotify.RecentlyPlayedOptions{
			Limit:        50,
			AfterEpochMs: afterTime,
		})
		if err != nil {
			log.Fatalf("could not get recently played: %v", err)
		}
		if len(recentlyPlayed) > 0 {
			afterTime = recentlyPlayed[0].PlayedAt.UnixMilli() // First played track time

			if len(recentlyPlayed) > decayThreshold {
				// Decrement counts for tracks not in the most recently played list
				for trackID := range fmap {
					found := false
					for _, item := range recentlyPlayed {
						if item.Track.ID == trackID {
							found = true
							break
						}
					}
					if !found {
						fmap[trackID]--
						if fmap[trackID] <= 0 {
							delete(fmap, trackID)
							fmt.Printf("Removed track from fmap: %s\n", trackID)
							// Remove the track from the "Repeats" playlist here, if it exists
							_, err := client.RemoveTracksFromPlaylist(context.Background(), repeatsPlaylist.ID, trackID)
							if err != nil {
								log.Printf("could not remove track from playlist: %v", err)
							}
							fmt.Printf("Removed track from playlist %s", trackID)
						}
					}
				}
			}
			log.Printf("afterTime %v", afterTime)

			for _, item := range recentlyPlayed {
				trackID := item.Track.ID
				fmap[trackID]++
				if fmap[trackID] >= fmapLimit {
					fmap[trackID] = fmapLimit //Limit to avoid never deleting

				}

				if fmap[trackID] == validListenTimes {
					// Check if track exists in playlist
					trackExists := false
					playlistItem, err := client.GetPlaylistItems(context.Background(), repeatsPlaylist.ID)
					if err != nil {
						log.Printf("could not get playlist tracks: %v", err)
						continue
					}
					for _, playlistTrack := range playlistItem.Items {
						if playlistTrack.Track.Track.ID == item.Track.ID {
							trackExists = true
							break
						}
					}

					if !trackExists {
						_, err := client.AddTracksToPlaylist(context.Background(), repeatsPlaylist.ID, item.Track.ID)
						if err != nil {
							log.Printf("could not add track to playlist: %v", err)
						}
						fmt.Printf("Added track: %s\n", item.Track.Name)
					}
				}
			}

		}
	}

}
