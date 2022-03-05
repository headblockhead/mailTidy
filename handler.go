package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"

	gomail "github.com/emersion/go-message/mail"
	"github.com/headblockhead/mailtidy/cal"
)

type Action string

const ActionNone Action = ""
const ActionDelete Action = "DELETE"

type Handler interface {
	Handle(message Message) (action Action, err error)
}

func NewMessage(seqNum uint32, mr *gomail.Reader) (msg Message, err error) {
	msg.SeqNum = seqNum
	msg.Subject, err = mr.Header.Subject()
	if err != nil {
		err = fmt.Errorf("message: could not get subject: %w", err)
		return
	}
	msg.From, err = mr.Header.AddressList("From")
	if err != nil {
		err = fmt.Errorf("message: could not get from: %w", err)
		return
	}
	msg.Date, _ = mr.Header.Date()
	msg.To, _ = mr.Header.AddressList("To")

	var sb strings.Builder
	for {
		var p *gomail.Part
		p, err = mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			err = fmt.Errorf("message: could not get part: %w", err)
			return
		}
		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			// The header is a message.
			var b []byte
			b, err = ioutil.ReadAll(p.Body)
			if err != nil {
				err = fmt.Errorf("message: could not read inline header: %w", err)
				return
			}
			sb.WriteString(string(b))
		case *gomail.AttachmentHeader:
			// The header is an attachment.
			var attachment Attachment
			attachment.FileName, err = h.Filename()
			if err != nil {
				err = fmt.Errorf("message: could not read attachment filename: %w", err)
				return
			}
			attachment.Body, err = ioutil.ReadAll(p.Body)
			if err != nil {
				err = fmt.Errorf("message: could not read attachment body: %w", err)
				return
			}
		}
	}

	return
}

type Message struct {
	SeqNum      uint32
	Subject     string
	From        []*mail.Address
	To          []*mail.Address
	Date        time.Time
	Body        string
	Attachments []Attachment
}

func (msg Message) String() string {
	return fmt.Sprintf(`Date: %v
From: %v
To: %v
Subject: %v
`, msg.Date, msg.From, msg.To, msg.Subject)
}

type Attachment struct {
	FileName string
	Body     []byte
}

type SecurityAlertHandler struct {
}

func (h SecurityAlertHandler) Handle(msg Message) (action Action, err error) {
	if !strings.Contains(msg.Subject, "Security alert") && strings.EqualFold(msg.From[0].Address, "no-reply@accounts.google.com") {
		return ActionNone, nil
	}
	if getInput("Do you want to delete this email? (Y/N)") != "Y" {
		return ActionNone, nil
	}
	return ActionDelete, nil
}

type FailedMessageSendHandler struct {
}

func (h FailedMessageSendHandler) Handle(msg Message) (action Action, err error) {
	if !strings.EqualFold(msg.From[0].Name, "Mail Delivery Subsystem") {
		return ActionNone, nil
	}
	if getInput("Do you want to delete this email? (Y/N)") != "Y" {
		return ActionNone, nil
	}
	return ActionDelete, nil
}

type CalendarHandler struct {
}

var calendarSubjectRegex = regexp.MustCompile(`.*@.*\(GMT\) \(.*\)`)

func (h CalendarHandler) Handle(msg Message) (action Action, err error) {
	for _, attachment := range msg.Attachments {
		if !strings.HasSuffix(attachment.FileName, ".ics") {
			continue
		}
		matches := calendarSubjectRegex.MatchString(msg.Subject)
		if !matches {
			log.Println("This message is not a google calendar invite. Skipping ICS installation.")
			continue
		}

		err = os.Mkdir("/tmp/golangmail", 0755)
		if err != nil {
			log.Println(err)
		}
		err = ioutil.WriteFile("/tmp/golangmail/temp.ics", c, 0777)
		if err != nil {
			log.Fatal(err)
		}
		shouldInstall := true
		if !force && !skip {
			log.Println("Do you want to install the calendar attachment in this email? (Y/N) ")
			var input string
			fmt.Scanln(&input)
			shouldInstall = strings.EqualFold(input, "y")
		}
		if shouldInstall {
			cal.InstallFILE("/tmp/golangmail/temp.ics")
		}
	}
}

func getInput(question string) (answer string) {
	fmt.Println(question)
	fmt.Scanln(&answer)
	answer = strings.ToUpper(answer)
	return
}
