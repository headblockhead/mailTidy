package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"

	_ "github.com/emersion/go-message/charset"
)

type Credentials struct {
	User string
	Pass string
	Serv string
	Mail string
}

func main() {
	creds := Credentials{}
	user := flag.String("user", "", "Username. Often formatted as \"username@example.com\"")
	pass := flag.String("pass", "", "Password.")
	serv := flag.String("serv", "", "Server. The IMAP server to connect to. Include the port. Example: \"imap.example.com:993\"")
	mail := flag.String("mail", "", "Mailbox. The Mailbox to scan. Example: \"INBOX\"")
	flag.Parse()
	creds.User = *user
	creds.Pass = *pass
	creds.Serv = *serv
	creds.Mail = *mail
	file, err := os.Open("credentials.json")
	if err != nil {
		fmt.Println("No credentials file found.")
	} else {
		defer file.Close()
		decode := json.NewDecoder(file)
		err = decode.Decode(&creds)
		if err != nil {
			fmt.Println(err)
		}
	}
	if creds.User == "" || creds.Pass == "" || creds.Serv == "" || creds.Mail == "" {
		fmt.Println("Missing arguments! Please include:")
		fmt.Println("Your username: (\"-user username@example.com\")")
		fmt.Println("Your password: (\"-pass yourpassword\")")
		fmt.Println("Your email server: (\"-serv imap.example.com:993\")")
		fmt.Println("Your mailbox: (\"-mail INBOX\")")
		fmt.Println("If you can, place theese in the credentials.json file instead. (see headblockhead.com/files/examples/mailtidy/credentials.json)")
		fmt.Println("For other arguments that are not required, use \"-help\"")
		return
	}

	err = process(creds)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func process(creds Credentials) (err error) {
	fmt.Println("Connecting to server...")
	c, err := client.DialTLS(creds.Serv, nil)
	if err != nil {
		return fmt.Errorf("error connecting: %w", err)
	}
	fmt.Println("Connected")

	// Don't forget to logout.
	defer c.Logout()

	// Login.
	if err := c.Login(creds.User, creds.Pass); err != nil {
		return fmt.Errorf("error logging in: %w", err)
	}
	fmt.Println("Logged in")

	// Select the mailbox.
	mbox, err := c.Select(creds.Mail, false)
	if err != nil {
		return fmt.Errorf("could not select the mailbox: %w", err)
	}
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages == 0 {
		return
	}
	if to > 100 {
		to = 100
	}

	handlers := []Handler{
		SecurityAlertHandler{},
		FailedMessageSendHandler{},
		CalendarHandler{},
		ExpiredEventHandler{},
	}
	act := NewDefaultActions()

	// Get all email in the inbox.
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)
	var section imap.BodySectionName
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 10)
	var channelError error
	go func() {
		channelError = c.Fetch(seqset, items, messages)
	}()

	// Loop through all messages.
	for msg := range messages {
		if msg == nil {
			return fmt.Errorf("server didn't return a message")
		}
		r := msg.GetBody(&section)
		if r == nil {
			return fmt.Errorf("server didn't return message body")
		}
		// Create a new mail reader.
		mr, err := mail.CreateReader(r)
		if err != nil {
			fmt.Printf("failed to read body: %v", err)
			continue
		}
		email, err := NewMessage(msg.SeqNum, mr)
		if err != nil {
			fmt.Printf("failed to read email: %v", err)
			continue
		}
		// Print the email. NewLine before it.
		fmt.Printf("\n" + email.String())
		// Process it.
		for _, h := range handlers {
			err := h.Handle(email, act)
			if err != nil {
				fmt.Printf("failed to process email: %v", err)
			}
		}
	}
	if channelError != nil {
		return fmt.Errorf("failed to fetch: %w", channelError)
	}

	// Delete all the deletion-flagged messages.
	if len(act.MessagesToDelete.Set) > 0 {
		item := imap.FormatFlagsOp(imap.AddFlags, true)
		flags := []interface{}{imap.DeletedFlag}
		if err := c.Store(act.MessagesToDelete, item, flags, nil); err != nil {
			return fmt.Errorf("failed to mark the message for deletion: %w", err)
		}
		if err := c.Expunge(nil); err != nil {
			return fmt.Errorf("failed to apply deletions: %w", err)
		}
	}
	// Open deferred links.
	fmt.Println("Opening all listed links... This may lag your computer.")
	for _, url := range act.LinksToOpen {
		act.OpenBrowser(url)
	}
	return nil
}
