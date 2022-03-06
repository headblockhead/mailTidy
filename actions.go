package main

import (
	"fmt"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/headblockhead/mailtidy/cal"
	"github.com/pkg/browser"
)

type Actions interface {
	Delete(msg Message)
	Print(s string)
	GetInput(prompt string) string
	ImportCalendarEvent(ics string) (link string, err error)
	OpenBrowser(link string) error
	OpenBrowserLater(link string)
}

type DefaultActions struct {
	MessagesToDelete *imap.SeqSet
	LinksToOpen      []string
}

func (da *DefaultActions) Delete(msg Message) {
	if da.MessagesToDelete == nil {
		da.MessagesToDelete = new(imap.SeqSet)
	}
	da.MessagesToDelete.AddNum(msg.SeqNum)
}

func (da *DefaultActions) Print(s string) {
	fmt.Println(s)
}

func (da *DefaultActions) GetInput(prompt string) (answer string) {
	fmt.Println(prompt)
	fmt.Scanln(&answer)
	answer = strings.ToUpper(answer)
	return
}

func (da *DefaultActions) ImportCalendarEvent(ics string) (eventLink string, err error) {
	return cal.Import(ics)
}

func (da *DefaultActions) OpenBrowser(link string) error {
	return browser.OpenURL(link)
}

func (da *DefaultActions) OpenBrowserLater(link string) {
	da.LinksToOpen = append(da.LinksToOpen, link)
}
