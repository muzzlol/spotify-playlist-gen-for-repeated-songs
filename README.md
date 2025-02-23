# spotify-playlist-gen-for-repeated-songs

A Go application that automatically generates and maintains a Spotify playlist based on your listening habits. It tracks your recently played songs and adds them to a "Repeats" playlist when you listen to them multiple times. If you stop listening to a song for a while, it will be automatically removed from the playlist.

## Features

- Automatically creates and manages a "Repeats" playlist
- Adds songs to the playlist after they've been played multiple times
- Removes songs from the playlist when they haven't been played recently
- Runs continuously with configurable polling intervals

## Prerequisites

1. Go 1.20 or later installed on your system
2. A Spotify Developer account and API credentials

## Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/muzzlol/spotify-playlist-gen-for-repeated-songs.git
   cd spotify-playlist-gen-for-repeated-songs
   ```

2. Create a Spotify application:
   - Go to [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   - Create a new application
   - Add `http://localhost:8080/auth/spotify/callback` as a redirect URI in your application settings

3. Set up environment variables:
   - Copy the example environment file:
     ```bash
     cp .env.example .env
     ```
   - Edit `.env` and add your Spotify application credentials:
     ```
     SPOTIFY_ID=your_client_id
     SPOTIFY_SECRET=your_client_secret
     SPOTIFY_REDIRECT_URI=http://localhost:8080/auth/spotify/callback
     ```

## Running the Application

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Run the application:
   ```bash
   go run .
   ```

3. When the application starts, it will display a URL. Open this URL in your web browser to authenticate with Spotify.

4. After successful authentication, the application will:
   - Create a "Repeats" playlist if it doesn't exist
   - Start monitoring your listening history
   - Automatically add songs you frequently listen to
   - Remove songs you haven't listened to recently

## Configuration

The application's behavior can be customized through the `config.yaml` file:

```yaml
# Duration between API calls (in hours)
poll_interval: 2

# Number of plays required to add track to playlist
valid_listenTimes: 3

# Maximum play count limit for songs
fmap_limit: 7

# Threshold value for decay process
decay_threshold: 15
```
