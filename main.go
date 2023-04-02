package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

const authURL = "https://accounts.spotify.com/authorize"

var (
	clientID     = os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	state        = os.Getenv("SPOTIFY_STATE")

	redirectURI = "http://localhost:8080/callback"
	scopes      = []string{
		"user-modify-playback-state",
		"user-read-playback-state",
	}
	auth = spotify.NewAuthenticator(redirectURI, scopes...)
)

func getSkipInterval() int {
	reader := bufio.NewReader(os.Stdin)

	// Ask the user to input the skip interval in seconds
	fmt.Print("Enter the skip interval in seconds: ")
	skipIntervalStr, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error reading input: %s", err)
	}

	// Convert the skip interval to an integer
	skipInterval, err := strconv.Atoi(strings.TrimSpace(skipIntervalStr))
	if err != nil {
		log.Fatalf("Error converting skip interval to integer: %s", err)
	}

	return skipInterval
}

func getAuthToken() *oauth2.Token {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}

	// Get the URL to redirect the user to for authorization
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Printf("Please log in to Spotify by visiting the following page in your browser:\n\n%s\n\n", authURL)

	// Ask the user to input the authorization code from the browser
	fmt.Print("Enter the redirect URL from the browser: ")
	reader := bufio.NewReader(os.Stdin)
	redirectedURL, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error reading input: %s", err)
	}

	// Parse the redirect URL and extract the authorization code
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(redirectedURL))
	if err != nil {
		log.Fatalf("Error parsing the URL: %s", err)
	}
	authCode := parsedURL.Query().Get("code")

	// Exchange the authorization code for an access token
	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Error exchanging code for token: %s", err)
	}

	return token
}

func main() {
	if clientID == "" || clientSecret == "" || state == "" {
		log.Fatalf("Error: Missing environment variables (SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET, SPOTIFY_STATE)")
	}

	skipInterval := time.Duration(getSkipInterval()) * time.Second

	token := getAuthToken()

	// Create an authenticated Spotify client
	client := auth.NewClient(token)

	for {
		// Get the current playback state
		playback, err := client.PlayerState()
		if err != nil {
			log.Fatalf("Error getting playback state: %v", err)
		}

		if playback.Item == nil {
			fmt.Println("No track currently playing.")
			time.Sleep(1 * time.Second)
			continue
		}

		// Get the current time elapsed in the track
		elapsedTime := time.Duration(playback.Progress) * time.Millisecond

		// Check if the desired time elapsed has been reached
		if elapsedTime >= skipInterval {
			// Skip to the next track
			err = client.Next()
			if err != nil {
				log.Fatalf("Error skipping to the next track: %v", err)
			}
			time.Sleep(1 * time.Second)
		} else {
			// Sleep for a short interval before checking again
			time.Sleep(1 * time.Second)
		}
	}
}
