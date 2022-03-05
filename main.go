package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"

	_ "github.com/emersion/go-message/charset"
)

type Credentials struct {
	User string
	Pass string
	Serv string
}

func main() {
	creds := Credentials{}
	user := flag.String("user", "", "Username. Often formatted as \"username@example.com\"")
	pass := flag.String("pass", "", "Password.")
	serv := flag.String("serv", "", "Server. The IMAP server to connect to. Include the port. Example: \"imap.example.com:993\"")
	force := flag.Bool("f", false, "Force: Do not question the user. Delete emails without asking first.")
	skip := flag.Bool("s", false, "Skip: Do not interrupt the program. Automatically ignore all unresponded calendar invites. Delete emails without asking first.")
	flag.Parse()
	creds.User = *user
	creds.Pass = *pass
	creds.Serv = *serv
	file, err := os.Open("credentials.json")
	if err != nil {
		log.Println("No credentials file found.")
	} else {
		defer file.Close()
		decode := json.NewDecoder(file)
		err = decode.Decode(&creds)
		if err != nil {
			log.Println(err)
		}
	}
	if creds.User == "" || creds.Pass == "" || creds.Serv == "" {
		log.Println("Missing arguments! Please include:")
		log.Println("Your username: (\"-user username@example.com\")")
		log.Println("Your password: (\"-pass yourpassword\")")
		log.Println("And your email server: (\"-serv imap.example.com:993\")")
		log.Println("If you can, place theese in the credentials.json file instead. (see headblockhead.com/files/examples/mailtidy/credentials.json)")
		log.Println("For other arguments that are not required, use \"-help\"")
		return
	}
	connectToServer(creds, *force, *skip)
}

func connectToServer(creds Credentials, force, skip bool) {
	log.Println("Connecting to server...")
	c, err := client.DialTLS(creds.Serv, nil)
	if err != nil {
		log.Fatalf("error connecting: %v", err)
	}
	log.Println("Connected")

	// Don't forget to logout
	defer c.Logout()

	// Login
	if err := c.Login(creds.User, creds.Pass); err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in")

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}

	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages == 0 {
		log.Fatal("No message in mailbox")
	}
	if to > 100 {
		to = 100
	}

	// Get all email in the inbox
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)
	var section imap.BodySectionName
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 10)
	go func() {
		if err := c.Fetch(seqset, items, messages); err != nil {
			log.Fatal(err)
		}
	}()

	messagesToDelete := new(imap.SeqSet)

	// Loop through all messages.
	for msg := range messages {
		if msg == nil {
			log.Fatal("Server didn't return a message")
		}

		r := msg.GetBody(&section)
		if r == nil {
			log.Fatal("Server didn't return the message's body")
		}

		// Create a new mail reader
		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Printf("failed to read body: %v", err)
			continue
		}
		subject, err := mr.Header.Subject()
		if err != nil {
			log.Fatalln("Could not get subject of message")
		}
		from, err := mr.Header.AddressList("From")
		if err != nil {
			log.Fatalln("Could not get subject of message")
		}
		if strings.Contains(subject, "Security alert") && strings.EqualFold(from[0].Address, "no-reply@accounts.google.com") {
			shouldDelete := true
			if !force && !skip {
				// Display some info about the message
				header := mr.Header
				if date, err := header.Date(); err == nil {
					// When the message was sent
					log.Println("Date:", date)
				}
				if from, err := header.AddressList("From"); err == nil {
					// Where the message was from
					log.Println("From:", from)
				}
				if to, err := header.AddressList("To"); err == nil {
					// Who the message was to
					log.Println("To:", to)
				}
				if subject, err := header.Subject(); err == nil {
					// What the message is about
					log.Println("Subject:", subject)
				}

				log.Println("Do you want to delete this email? (Y/N) ")
				var input string
				fmt.Scanln(&input)
				shouldDelete = strings.EqualFold(input, "y")
			}
			if shouldDelete {
				log.Printf("Setting deleted flag on msg %d", msg.SeqNum)
				messagesToDelete.AddNum(msg.SeqNum)
			}
		}
		if strings.EqualFold(from[0].Name, "Mail Delivery Subsystem") {
			shouldDelete := true
			if !force && !skip {
				// Display some info about the message
				header := mr.Header
				if date, err := header.Date(); err == nil {
					// When the message was sent
					log.Println("Date:", date)
				}
				if from, err := header.AddressList("From"); err == nil {
					// Where the message was from
					log.Println("From:", from)
				}
				if to, err := header.AddressList("To"); err == nil {
					// Who the message was to
					log.Println("To:", to)
				}
				if subject, err := header.Subject(); err == nil {
					// What the message is about
					log.Println("Subject:", subject)
				}

				log.Println("Do you want to delete this email? (Y/N) ")
				var input string
				fmt.Scanln(&input)
				shouldDelete = strings.EqualFold(input, "y")
			}
			if shouldDelete {
				log.Printf("Setting deleted flag on msg %d", msg.SeqNum)
				messagesToDelete.AddNum(msg.SeqNum)
			}

		}
		// Combine all the message parts into a large string

		var sb strings.Builder

		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}

			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				// The header is a message
				b, _ := ioutil.ReadAll(p.Body)
				sb.WriteString(string(b))
			case *mail.AttachmentHeader:
				// The header is an attachment
				filename, _ := h.Filename()
				if !strings.Contains(filename, ".ics") {
					continue
				}
			}
		}
		// Make a HTML document from the message contents
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(sb.String()))
		if err != nil {
			log.Fatal(err)
		}
		// Get a tag named "time", then get the first occurence, then find the unformatted string containing the time
		timegot, _ := doc.Find("time").First().Attr("datetime")
		layout := "20060102T150405Z"
		if timegot == "" {
			// log.Println("No event start time found for this message.")
		} else {
			// Format the time string into a variable
			t, err := time.Parse(layout, timegot)
			if err != nil {
				log.Println(err)
			}
			log.Println("Got event start date:", t)
		}
		// Get a tag name "time", then get the last occurence,  then find the unformatted string containing the time
		timegotend, _ := doc.Find("time").Last().Attr("datetime")
		if timegotend == "" {
			header := mr.Header
			if date, err := header.Date(); err == nil {
				// When the message was sent
				log.Println("Date:", date)
			}
			if from, err := header.AddressList("From"); err == nil {
				// Where the message was from
				log.Println("From:", from)
			}
			if subject, err := header.Subject(); err == nil {
				// What the message is about
				log.Println("Subject:", subject)
			}
			log.Println("No event end time found for this message.")
		} else {
			// Format the time string into a variable
			tend, err := time.Parse(layout, timegotend)
			if err != nil {
				log.Println(err)
			}
			log.Println("Got event end date:", tend)

			// If the end date of the even has already passed
			if tend.Before(time.Now()) {
				log.Println("End date of the event in the past, deleting email.")
				shouldDelete := true
				if !force && !skip {
					// Display some info about the message
					header := mr.Header
					if date, err := header.Date(); err == nil {
						// When the message was sent
						log.Println("Date:", date)
					}
					if from, err := header.AddressList("From"); err == nil {
						// Where the message was from
						log.Println("From:", from)
					}
					if to, err := header.AddressList("To"); err == nil {
						// Who the message was to
						log.Println("To:", to)
					}
					if subject, err := header.Subject(); err == nil {
						// What the message is about
						log.Println("Subject:", subject)
					}
					log.Println("Do you want to delete this email? (Y/N) ")
					var input string
					fmt.Scanln(&input)
					shouldDelete = strings.EqualFold(input, "y")
				}
				if shouldDelete {
					log.Printf("Setting deleted flag on msg %d", msg.SeqNum)
					messagesToDelete.AddNum(msg.SeqNum)
				}
			} else if strings.Contains(sb.String(), "https://calendar.google.com/calendar/event?action=RESPOND") {
				// Display some info about the message
				header := mr.Header

				if date, err := header.Date(); err == nil {
					// When the message was sent
					log.Println("Date:", date)
				}
				if from, err := header.AddressList("From"); err == nil {
					// Where the message was from
					log.Println("From:", from)
				}
				if to, err := header.AddressList("To"); err == nil {
					// Who the message was to
					log.Println("To:", to)
				}
				if subject, err := header.Subject(); err == nil {
					// What the message is about
					log.Println("Subject:", subject)
				}

				respond(doc, true, skip, c, msg)
			} else {
				log.Println("No calendar response found for this message.")
			}
		}

		log.Println("")
	}

	// Flagging all the deleted messages.
	if len(messagesToDelete.Set) > 0 {
		item := imap.FormatFlagsOp(imap.AddFlags, true)
		flags := []interface{}{imap.DeletedFlag}
		if err := c.Store(messagesToDelete, item, flags, nil); err != nil {
			log.Printf("Failed to mark the message for deletion: %v", err)
			os.Exit(1)
		}
		if err := c.Expunge(nil); err != nil {
			log.Println("Failed to apply deletions.")
			os.Exit(1)
		}
	}
	log.Println("Opening all listed links... This may lag your computer.")
	for _, url := range urlstoopen {
		openbrowsernow(url)
	}
}

var urlstoopen []string

func openbrowser(url string) {
	log.Println("Adding to link list.")
	urlstoopen = append(urlstoopen, url)
}
func openbrowsernow(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}
func respond(doc *goquery.Document, first bool, skipval bool, c *client.Client, msg *imap.Message) {
	if first {
		log.Println("Calendar response found for this message!")
	}
	if !skipval {
		log.Println("Yes (Y), No (N), Mabye (M), Details (D), Ignore (I) or Delete (X)")
		var opened bool = false
		var detailsshown bool = false
		var response string
		fmt.Scanln(&response)
		if strings.EqualFold(response, "y") {
			doc.Find("a").Each(func(i int, ul *goquery.Selection) {
				link, _ := ul.Attr("href")
				if strings.Contains(link, "event?action=RESPOND") && strings.Contains(link, "rst=1") {
					openbrowser(link)
					opened = true
				}
			})
		}
		if strings.EqualFold(response, "m") {
			doc.Find("a").Each(func(i int, ul *goquery.Selection) {
				link, _ := ul.Attr("href")
				if strings.Contains(link, "event?action=RESPOND") && strings.Contains(link, "rst=3") {
					openbrowser(link)
					opened = true
				}
			})
		}
		if strings.EqualFold(response, "n") {
			doc.Find("a").Each(func(i int, ul *goquery.Selection) {
				link, _ := ul.Attr("href")
				if strings.Contains(link, "event?action=RESPOND") && strings.Contains(link, "rst=2") {
					openbrowser(link)
					opened = true
				}
			})
		}
		if strings.EqualFold(response, "d") {
			doc.Find("a").Each(func(i int, ul *goquery.Selection) {
				link, _ := ul.Attr("href")
				if strings.Contains(link, "event?action=VIEW") && !detailsshown {
					log.Println("Displaying further details...")
					openbrowsernow(link)
					detailsshown = true
				}
			})
		}
		if strings.EqualFold(response, "x") {
			AMessageToDelete := new(imap.SeqSet)
			AMessageToDelete.AddNum(msg.SeqNum)
			item := imap.FormatFlagsOp(imap.AddFlags, true)
			flags := []interface{}{imap.DeletedFlag}
			if err := c.Store(AMessageToDelete, item, flags, nil); err != nil {
				log.Printf("Failed to mark the message for deletion: %v", err)
				os.Exit(1)
			}
			if err := c.Expunge(nil); err != nil {
				log.Println("Failed to apply deletions.")
				os.Exit(1)
			}
			opened = true
		}
		if strings.EqualFold(response, "i") {
			return
		}
		if !opened {
			respond(doc, false, skipval, c, msg)
		}
	} else {
		return
	}
}
