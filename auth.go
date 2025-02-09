package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

var (
	auth  *spotifyauth.Authenticator
	state = "replacethiswitharandomstringinprod"
	ch    = make(chan *spotify.Client)
)

func init() {
	// Load the .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	clientID := os.Getenv("SPOTIFY_ID")
	clientSecret := os.Getenv("SPOTIFY_SECRET")
	redirectURI := os.Getenv("SPOTIFY_REDIRECT_URI")
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		log.Fatal("Missing required Spotify credentials in environment variables")
	}

	auth = spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistReadCollaborative,
			spotifyauth.ScopePlaylistModifyPublic,
			spotifyauth.ScopePlaylistModifyPrivate,
			spotifyauth.ScopeUserReadRecentlyPlayed,
		),
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithClientSecret(clientSecret),
	)
}

func spotifyLoginHandler(w http.ResponseWriter, r *http.Request) {
	url := auth.AuthURL(state)                 // Login url sent with defined scopes, state and redirect uri
	http.Redirect(w, r, url, http.StatusFound) // redirect to login url
}

// handler for redirect uri i.e after login
func spotifyCallbackHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	// use token to get authenticated client
	client := spotify.New(auth.Client(r.Context(), token)) // returns pointer to authenticated client
	ch <- client
}

func waitForAuthentication() *spotify.Client {
	return <-ch
}
