package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/zmb3/spotify/v2"
	"gopkg.in/yaml.v3"
)

type Config struct {
	PollInterval     int `yaml:"pollInterval"`
	ValidListenTimes int `yaml:"validListenTimes"`
	FmapLimit        int `yaml:"fmapLimit"`
	DecayThreshold   int `yaml:"decayThreshold"`
}

var (
	pollInterval     time.Duration
	validListenTimes int
	fmapLimit        int
	decayThreshold   int
	afterTime        int64  // unix after value
	lastSnapshot     string // Track the last snapshot ID of the playlist
)

func loadConfig() {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("error reading config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatalf("error parsing config file: %v", err)
	}

	pollInterval = time.Duration(config.PollInterval) * time.Hour
	validListenTimes = config.ValidListenTimes
	fmapLimit = config.FmapLimit
	decayThreshold = config.DecayThreshold

	log.Printf("Loaded configuration: pollInterval=%v, validListenTimes=%d, fmapLimit=%d, decayThreshold=%d",
		pollInterval, validListenTimes, fmapLimit, decayThreshold)
}

func main() {
	loadConfig()
	http.HandleFunc("/auth/spotify/login", spotifyLoginHandler)
	http.HandleFunc("/auth/spotify/callback", spotifyCallbackHandler)

	fmt.Printf("\nPlease visit this URL to authenticate with Spotify: http://localhost:8080/auth/spotify/login\n\n")

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

	// run core once immediately then move to ticker loop
	core(fmap, client, repeatsPlaylist)

	// TODO : replace with switch case logic cause wtf, ugly
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	log.Printf("Started ticker with interval: %v\n", pollInterval)
	// function runs every time ticker ticks otherwise nothing happens between ticks
	for tick := range ticker.C {
		log.Printf("Ticker ticked at: %v\n", tick)
		core(fmap, client, repeatsPlaylist)
		log.Println("Ticker finished")
	}
}

// syncFmapWithSnapshot will check if the playlist snapshot has changed.
// If it has, the function scans the playlist items (using the snapshot id)
// and updates the internal frequency map accordingly.
func syncFmapWithSnapshot(ctx context.Context, fmap map[spotify.ID]int, client *spotify.Client,
	playlist *spotify.SimplePlaylist) {

	// Fetch the playlist items
	FullPlaylist, err := client.GetPlaylist(ctx, playlist.ID)
	if err != nil {
		log.Printf("Error fetching playlist items for sync: %v", err)
		return
	}

	// If the snapshot ID has not changed, then nothing has been modified.
	if lastSnapshot == FullPlaylist.SnapshotID {
		log.Println("Playlist snapshot unchanged. No sync necessary.")
		return
	}

	log.Printf("Detected playlist snapshot change: Old: %q, New: %q\n", lastSnapshot, FullPlaylist.SnapshotID)
	// Build a set of track IDs in the current playlist.
	currentTracks := make(map[spotify.ID]bool)

	// Initialize variables for pagination
	offset := 0
	limit := 50 // Spotify's default limit

	// Loop until we've fetched all tracks
	for {
		playlistTracks, err := client.GetPlaylistItems(ctx, playlist.ID, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			log.Printf("Error fetching playlist items at offset %d: %v", offset, err)
			return
		}

		// Process tracks from current page
		for _, item := range playlistTracks.Items {
			trackID := item.Track.Track.ID
			currentTracks[trackID] = true

			// If the track is in the playlist but not in the fmap, assume it was manually added.
			if _, exists := fmap[trackID]; !exists {
				fmap[trackID] = validListenTimes
				log.Printf("Manual addition: Track %s found in playlist. Adding to fmap with count %d.\n", trackID, validListenTimes)
			}
		}

		// Check if we've reached the end of the playlist
		if len(playlistTracks.Items) < limit {
			break
		}

		// Move to next page
		offset += limit
	}

	// Check for manual removals: if a track exists in fmap but is no longer in the playlist,
	// then remove it so it won't be re-added automatically, but only if it had reached validListenTimes
	for trackID := range fmap {
		if _, found := currentTracks[trackID]; !found {
			// Only remove from fmap if the track had reached validListenTimes (was previously in playlist)
			if fmap[trackID] >= validListenTimes {
				log.Printf("Manual removal: Track %s missing from playlist and had reached validListenTimes. Removing from fmap.\n", trackID)
				delete(fmap, trackID)
			} else {
				log.Printf("Track %s missing from playlist but hasn't reached validListenTimes (%d/%d). Keeping in fmap.\n", trackID, fmap[trackID], validListenTimes)
			}
		}
	}

	// Finally, update the lastSnapshot variable.
	lastSnapshot = FullPlaylist.SnapshotID
	log.Printf("Sync complete. Updated snapshot id to %q.\n", lastSnapshot)
}

func core(fmap map[spotify.ID]int, client *spotify.Client, repeatsPlaylist *spotify.SimplePlaylist) {
	log.Printf("----- Core function running -----\n")

	// Sync manual changes using snapshot ID
	syncFmapWithSnapshot(context.Background(), fmap, client, repeatsPlaylist)

	recentlyPlayed, err := client.PlayerRecentlyPlayedOpt(context.Background(), &spotify.RecentlyPlayedOptions{
		Limit:        50,
		AfterEpochMs: afterTime,
	})
	if err != nil {
		log.Printf("could not get recently played: %v", err)
	}
	log.Printf("Fetched %d recently played tracks\n", len(recentlyPlayed))
	for index, item := range recentlyPlayed {
		log.Printf("%d: Recently played track: %s by %s", index, item.Track.Name, item.Track.Artists[0].Name)
	}
	if len(recentlyPlayed) > 0 {
		afterTime = max(afterTime, recentlyPlayed[0].PlayedAt.UnixMilli()) // First played track time
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
				offset := 0
				limit := 50 // Spotify's default limit

				// Loop until we've checked all tracks
				for {
					playlistItems, err := client.GetPlaylistItems(context.Background(), repeatsPlaylist.ID, spotify.Limit(limit), spotify.Offset(offset))
					if err != nil {
						log.Printf("could not get playlist tracks: %v", err)
						break
					}

					for _, playlistTrack := range playlistItems.Items {
						if playlistTrack.Track.Track.ID == item.Track.ID {
							trackExists = true
							log.Printf("Track %s already exists in playlist\n", trackID)
							break
						}
					}

					// If we found the track or reached the end of the playlist, stop searching
					if trackExists || len(playlistItems.Items) < limit {
						break
					}

					// Move to next page
					offset += limit
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
	log.Println("----- Core function finished -----")
}
