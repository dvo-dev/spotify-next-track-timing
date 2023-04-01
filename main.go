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
	skipInterval := getSkipInterval()

	token := getAuthToken()

	// Create an authenticated Spotify client
	client := auth.NewClient(token)

	// Create channels for communication
	trackChange := make(chan bool)
	pauseTimer := make(chan bool)
	resumeTimer := make(chan bool)

	// Goroutine to poll the current track in the background
	go func() {
		var currentTrackID spotify.ID
		var trackPaused bool
		for {
			// Get the current track
			playing, err := client.PlayerCurrentlyPlaying()
			if err != nil {
				log.Printf("Error getting currently playing track: %s", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if playing.Item != nil {
				if playing.Item.ID != currentTrackID {
					// If the track has changed, send a notification
					currentTrackID = playing.Item.ID
					trackChange <- true
				}

				if playing.Playing && trackPaused {
					trackPaused = false
					resumeTimer <- true
				} else if !playing.Playing && !trackPaused {
					trackPaused = true
					pauseTimer <- true
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Goroutine to skip tracks based on the interval and track change notifications
	go func() {
		timer := time.NewTimer(time.Duration(skipInterval) * time.Second)
		timer.Stop()
		var timerRunning bool

		for {
			select {
			case <-trackChange:
				// If the track has changed, reset the timer
				if !timerRunning {
					timer.Reset(time.Duration(skipInterval) * time.Second)
					timerRunning = true
				} else {
					timer.Reset(time.Duration(skipInterval) * time.Second)
				}
			case <-pauseTimer:
				// If the track is paused, stop the timer
				timer.Stop()
				timerRunning = false
			case <-resumeTimer:
				// If the track is resumed, restart the timer
				timer.Reset(time.Duration(skipInterval) * time.Second)
				timerRunning = true
			case <-timer.C:
				// Skip to the next track
				err := client.Next()
				if err != nil {
					log.Printf("Error skipping to the next track: %s", err)
				} else {
					fmt.Println("Skipped to the next track!")
				}
				timer.Reset(time.Duration(skipInterval) * time.Second)
			}
		}
	}()

	// Keep the program running
	select {}
}
