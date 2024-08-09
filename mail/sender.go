package mail

import (
	"fmt"
	"net/smtp"

	"github.com/jordan-wright/email"
)

type EmailSender interface {
	SendEmail(
		subject string,
		content string,
		to []string,
		cc []string,
		bcc []string,
		attachFiles []string,
	) error
}

type GmailSender struct {
	name              string
	fromEmailAddress  string
	fromEmailPassword string
}

func NewGamilSender(name string, fromEmailAddress string, fromEmailPassword string) EmailSender {
	return &GmailSender{
		name:              name,
		fromEmailAddress:  fromEmailAddress,
		fromEmailPassword: fromEmailPassword,
	}
}

func (sender *GmailSender) SendEmail(
	subject string,
	content string,
	to []string,
	cc []string,
	bcc []string,
	attachFiles []string,
) error {
	e := email.NewEmail()
	e.From = fmt.Sprintf("%s <%s>", sender.name, sender.fromEmailAddress)
	e.To = to
	e.Bcc = bcc
	e.Cc = cc
	e.Subject = subject
	e.HTML = []byte(content)

	for _, f := range attachFiles {
		_, err := e.AttachFile(f)
		if err != nil {
			return fmt.Errorf("failed to attch file %s: %w", f, err)
		}
	}

	return e.Send("smtp.gmail.com:587", smtp.PlainAuth("", sender.fromEmailAddress, sender.fromEmailPassword, "smtp.gmail.com"))
}
