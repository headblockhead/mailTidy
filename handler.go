package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Handler interface {
	Handle(message Message, act Actions) (err error)
}

type SecurityAlertHandler struct {
}

func (h SecurityAlertHandler) Handle(msg Message, act Actions) (err error) {
	if !strings.Contains(msg.Subject, "Security alert") || !strings.EqualFold(msg.From[0].Address, "no-reply@accounts.google.com") {
		return nil
	}
	if act.GetInput("securityalert: Do you want to delete this email? (Y/N)") != "Y" {
		return nil
	}
	act.Delete(msg)
	return nil
}

type FailedMessageSendHandler struct {
}

func (h FailedMessageSendHandler) Handle(msg Message, act Actions) (err error) {
	if !strings.EqualFold(msg.From[0].Name, "Mail Delivery Subsystem") {
		return nil
	}
	if act.GetInput("failedmessagesend: Do you want to delete this email? (Y/N)") != "Y" {
		return nil
	}
	act.Delete(msg)
	return nil
}

type CalendarHandler struct {
}

var calendarSubjectRegex = regexp.MustCompile(`.*@.*\(GMT\) \(.*\)`)

func (h CalendarHandler) Handle(msg Message, act Actions) (err error) {
	for _, attachment := range msg.Attachments {
		if !strings.HasSuffix(attachment.FileName, ".ics") {
			continue
		}
		matches := calendarSubjectRegex.MatchString(msg.Subject)
		if !matches {
			act.Print("This message is not a google calendar invite. Skipping ICS installation.")
			continue
		}
		if act.GetInput("Do you want to install the calendar attachment in this email? (Y/N)") == "Y" {
			link, err := act.ImportCalendarEvent(string(attachment.Body))
			if err != nil {
				return err
			}
			act.Print(fmt.Sprintf("Event created: %v", link))
		}
	}
	return
}

type ExpiredEventHandler struct {
}

func (h ExpiredEventHandler) Handle(msg Message, act Actions) (err error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(msg.Body))
	if err != nil {
		act.Print(fmt.Sprintf("expiredeventhandler: not a HTML message: %w", err))
	}
	// Get the start time.
	ts, _ := doc.Find("time").First().Attr("datetime")
	layout := "20060102T150405Z"
	if ts == "" {
		act.Print("No event start time found for this message.")
		return nil
	}
	// Format the time string into a variable.
	startTime, err := time.Parse(layout, ts)
	if err != nil {
		return fmt.Errorf("expiredeventhandler: invalid start time format: %w", err)
	}
	// Get a tag name "time", then get the last occurence,  then find the unformatted string containing the time.
	te, _ := doc.Find("time").Last().Attr("datetime")
	if te == "" {
		act.Print("No event end time found for this message.")
		return nil
	}
	// Format the time string into a variable.
	endTime, err := time.Parse(layout, te)
	if err != nil {
		return fmt.Errorf("expiredeventhandler: invalid end time format: %w", err)
	}
	act.Print(fmt.Sprintf("Start time: %v, end time: %v", startTime, endTime))

	// If the end date of the event has already passed.
	if endTime.Before(time.Now()) {
		if act.GetInput("Date of event is in the past, do you want to delete this email? (Y/N)") == "Y" {
			act.Delete(msg)
			return
		}
	} else if strings.Contains(msg.Body, "https://calendar.google.com/calendar/event?action=RESPOND") {
		h.respond(msg, doc, act)
		return
	}
	act.Print("No calendar response found for this message.")
	return
}

func (h ExpiredEventHandler) respond(msg Message, doc *goquery.Document, act Actions) {
	response := act.GetInput("There is a calendar event, respond with Yes (Y), No (N), Maybe (M), view Details (D), Ignore (I) or Delete (X)")
	if response == "I" {
		return
	}
	if response == "X" {
		act.Delete(msg)
		return
	}
	if strings.EqualFold(response, "D") {
		doc.Find("a").Each(func(i int, ul *goquery.Selection) {
			link, _ := ul.Attr("href")
			if strings.Contains(link, "event?action=VIEW") {
				act.OpenBrowser(link)
			} else {
				act.Print("Could not find details.")
			}
			h.respond(msg, doc, act)
			return
		})
	}
	var rst string
	switch response {
	case "Y":
		rst = "1"
	case "M":
		rst = "3"
	case "N":
		rst = "2"
	default:
		act.Print("Invalid input")
		h.respond(msg, doc, act)
		return
	}
	doc.Find("a").Each(func(i int, ul *goquery.Selection) {
		link, _ := ul.Attr("href")
		if strings.Contains(link, "event?action=RESPOND") && strings.Contains(link, "rst="+rst) {
			act.OpenBrowserLater(link)
		}
	})
}
