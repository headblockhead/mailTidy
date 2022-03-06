package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/mail"
	"strings"
	"time"

	gomail "github.com/emersion/go-message/mail"
)

type Message struct {
	SeqNum      uint32
	Subject     string
	From        []*mail.Address
	To          []*mail.Address
	Date        time.Time
	Body        string
	Attachments []Attachment
}

type Attachment struct {
	FileName string
	Body     []byte
}

func (msg Message) String() string {
	return fmt.Sprintf(`Date: %v
From: %v
To: %v
Subject: %v
`, msg.Date, msg.From, msg.To, msg.Subject)
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
			err = nil
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
	msg.Body = sb.String()
	return
}
