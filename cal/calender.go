package cal

import (
	"context"
	"encoding/json"
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
func SaveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func InstallFILE(path string) {
	ctx := context.Background()
	b, err := ioutil.ReadFile("cal/credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := GetClient(config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}
	// Refer to the Go quickstart on how to setup the environment:
	// https://developers.google.com/calendar/quickstart/go
	// Change the scope to calendar.CalendarScope and delete any stored credentials.

	input, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
	}

	s := strings.Split(strings.Split(string(input), "DTSTART:")[1], "\x0d\x0aDTEND")[0]
	starttime, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		log.Fatalln(err)
	}
	s = strings.Split(strings.Split(string(input), "DTEND:")[1], "\x0d\x0aDTSTAMP")[0]
	endtime, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		log.Fatalln(err)
	}
	GoogleDateTime := "2006-01-02T15:04:05-07:00"
	Formattedstarttime := starttime.Format(GoogleDateTime)
	Formattedendtime := endtime.Format(GoogleDateTime)
	descript := strings.Split(strings.Split(string(input), "DESCRIPTION:")[1], "\x0d\x0aLAST-MODIFIED:")[0]
	summar := strings.Split(strings.Split(string(input), "SUMMARY:")[1], "\x0d\x0aTRANSP:")[0]
	method := strings.Split(strings.Split(string(input), "METHOD:")[1], "\x0d\x0aBEGIN:")[0]
	fmt.Println()
	if method == "REQUEST" {
		event := &calendar.Event{
			Summary:     summar,
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
		calendarId := "primary"
		event, err = srv.Events.Insert(calendarId, event).Do()
		if err != nil {
			log.Fatalf("Unable to create event. %v\n", err)
		}
		fmt.Printf("Event created: %s\n", event.HtmlLink)
	} else {
		calendarId := "primary"
		thing := srv.Events.List(calendarId)
		if err != nil {
			log.Fatalf("Unable to create event. %v\n", err)
		}
		fmt.Printf("Event deleted: %s\n", thing.Header())
	}
}
