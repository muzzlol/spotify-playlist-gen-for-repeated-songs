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
	validListenTimes int           = 3               // Track add if plays exceed
	fmapLimit        int           = 7               // Limit plays for songs
	decayThreshold   int           = 5               // API calls are decremented if return exceeds
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
	log.Printf("Logged in as user ID: %s\n", user.ID)

	var repeatsPlaylist *spotify.SimplePlaylist

	playlists, err := client.GetPlaylistsForUser(context.Background(), user.ID)
	if err != nil {
		log.Fatalf("could not get playlists: %v", err)
	}
	log.Printf("Found %d playlists for user %s\n", len(playlists.Playlists), user.ID)

	playlistExists := false
	for _, playlist := range playlists.Playlists {
		if playlist.Name == "Repeats" {
			playlistExists = true
			repeatsPlaylist = &playlist
			log.Printf("Found 'Repeats' playlist with ID: %s\n", playlist.ID)
			break
		}
	}

	if !playlistExists {
		log.Println("'Repeats' playlist does not exist, creating...")
		playlist, err := client.CreatePlaylistForUser(context.Background(), user.ID, "Repeats", "Playlist for tracks on repeat", false, false)
		if err != nil {
			log.Fatalf("could not create playlist: %v", err)
		}
		repeatsPlaylist = &spotify.SimplePlaylist{
			Name: playlist.Name,
			ID:   playlist.ID,
		}
		fmt.Printf("Created playlist: %s\n", playlist.Name)
		log.Printf("Created 'Repeats' playlist with ID: %s\n", playlist.ID)
	}

	fmap := make(map[spotify.ID]int) // Store fmap with track id as spotify.ID
	log.Println("Initialized frequency map")

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	log.Printf("Started ticker with interval: %v\n", pollInterval)

	// function runs every time ticker ticks otherwise nothing happens between ticks
	for tick := range ticker.C {
		log.Printf("----- Ticker ticked at: %v -----\n", tick)
		recentlyPlayed, err := client.PlayerRecentlyPlayedOpt(context.Background(), &spotify.RecentlyPlayedOptions{
			Limit:        50,
			AfterEpochMs: afterTime,
		})
		if err != nil {
			log.Fatalf("could not get recently played: %v", err)
		}
		log.Printf("Fetched %d recently played tracks\n", len(recentlyPlayed))
		for index, item := range recentlyPlayed {
			log.Printf("%d: Recently played track: %s by %s", index, item.Track.Name, item.Track.Artists[0].Name)
		}
		if len(recentlyPlayed) > 0 {
			afterTime = recentlyPlayed[0].PlayedAt.UnixMilli() // First played track time
			log.Printf("Set afterTime to: %v\n", afterTime)

			if len(recentlyPlayed) > decayThreshold {
				log.Println("More than decayThreshold tracks, starting decay process")
				// Decrement counts for repeats pl tracks not in the most recently played list from fmap
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
						log.Printf("Track %s not found in recently played, decrementing count to %d\n", trackID, fmap[trackID])
						if fmap[trackID] <= 0 {
							delete(fmap, trackID)
							log.Printf("Track %s count reached 0, removing from fmap\n", trackID)

							// Remove the track from the "Repeats" playlist here, if it exists
							_, err := client.RemoveTracksFromPlaylist(context.Background(), repeatsPlaylist.ID, trackID)
							if err != nil {
								log.Printf("could not remove track %s from playlist: %v", trackID, err)
							} else {
								fmt.Printf("Removed track from playlist %s", trackID)
								log.Printf("Removed track %s from playlist\n", trackID)
							}
						}
					}
				}
			}

			for _, item := range recentlyPlayed {
				trackID := item.Track.ID
				fmap[trackID]++
				log.Printf("Track %s found in recently played, incrementing count to %d\n", trackID, fmap[trackID])
				if fmap[trackID] >= fmapLimit {
					fmap[trackID] = fmapLimit //Limit to avoid never deleting
					log.Printf("Track %s count reached fmapLimit of %d\n", trackID, fmapLimit)
				}

				if fmap[trackID] == validListenTimes {
					log.Printf("Track %s count reached validListenTimes of %d, checking playlist\n", trackID, validListenTimes)
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
							log.Printf("Track %s already exists in playlist\n", trackID)
							break
						}
					}

					if !trackExists {
						_, err := client.AddTracksToPlaylist(context.Background(), repeatsPlaylist.ID, item.Track.ID)
						if err != nil {
							log.Printf("could not add track to playlist: %v", err)
						}
						fmt.Printf("Added track: %s\n", item.Track.Name)
						log.Printf("Added track %s to playlist\n", trackID)
					}
				}
			}

		} else {
			log.Println("No recently played tracks found")
		}
		log.Println("----- Ticker finished -----")
	}

}
