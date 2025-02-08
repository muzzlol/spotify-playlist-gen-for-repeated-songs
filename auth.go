package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	spotifyauth "github.com/zmb3/spotify/v2/auth"

	"github.com/zmb3/spotify/v2"
)

var (
	auth         *spotifyauth.Authenticator
	clientID     = os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	redirectURI  = os.Getenv("SPOTIFY_REDIRECT_URI")
	state        = "replacethiswitharandomstringinprod"
	ch           = make(chan *spotify.Client)
)

func init() {
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
	fmt.Printf("Logged in as: %s", client)
	ch <- client
}

func waitForAuthentication() *spotify.Client {
	return <-ch
}
