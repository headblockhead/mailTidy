package cal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Retrieve a token, saves the token, then returns the generated client.
func GetClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "cal/token.json"
	tok, err := TokenFromFile(tokFile)
	if err != nil {
		tok = GetTokenFromWeb(config)
		SaveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func GetTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func TokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func SaveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("cal: unable to cache oauth token: %v", err)
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		return fmt.Errorf("cal: unable to encode oauth token: %v", err)
	}
	return err
}

func Import(ics string) (eventLink string, err error) {
	ctx := context.Background()
	b, err := ioutil.ReadFile("cal/credentials.json")
	if err != nil {
		return "", fmt.Errorf("import: unable to read client secret file: %w", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return "", fmt.Errorf("import: unable to parse client secret file to config: %w", err)
	}
	client := GetClient(config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("import: unable to create calendar client: %w", err)
	}
	// Refer to the Go quickstart on how to setup the environment:
	// https://developers.google.com/calendar/quickstart/go
	// Change the scope to calendar.CalendarScope and delete any stored credentials.
	s := strings.Split(strings.Split(ics, "DTSTART:")[1], "\x0d\x0aDTEND")[0]
	starttime, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		return "", fmt.Errorf("import: unable to parse start date: %w", err)
	}
	s = strings.Split(strings.Split(ics, "DTEND:")[1], "\x0d\x0aDTSTAMP")[0]
	endtime, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		return "", fmt.Errorf("import: unable to parse end date: %w", err)
	}
	GoogleDateTime := "2006-01-02T15:04:05-07:00"
	Formattedstarttime := starttime.Format(GoogleDateTime)
	Formattedendtime := endtime.Format(GoogleDateTime)
	descript := strings.Split(strings.Split(ics, "DESCRIPTION:")[1], "\x0d\x0aLAST-MODIFIED:")[0]
	summary := strings.Split(strings.Split(ics, "SUMMARY:")[1], "\x0d\x0aTRANSP:")[0]
	method := strings.Split(strings.Split(ics, "METHOD:")[1], "\x0d\x0aBEGIN:")[0]
	if method == "REQUEST" {
		event := &calendar.Event{
			Summary:     summary,
			Description: strings.ReplaceAll(descript, "\\n", "\n"),
			Start: &calendar.EventDateTime{
				DateTime: Formattedstarttime,
				// TimeZone: "America/Los_Angeles",
			},
			End: &calendar.EventDateTime{
				DateTime: Formattedendtime,
				// TimeZone: "America/Los_Angeles",
			},
			// Attendees: []*calendar.EventAttendee{
			// 	{Email: "lpage@example.com"},
			// 	{Email: "sbrin@example.com"},
			// },
		}
		calendarID := "primary"
		event, err = srv.Events.Insert(calendarID, event).Do()
		if err != nil {
			return "", fmt.Errorf("import: unable to create event: %w", err)
		}
		return event.HtmlLink, nil
	}
	return "", errors.New("import: unable to process ICS file")
}
