package mail

import (
	"testing"

	"github.com/shienlee73/simplebank/util"
	"github.com/stretchr/testify/require"
)

func TestSendEmailWithGamil(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config, err := util.LoadConfig("..")
	require.NoError(t, err)

	sender := NewGamilSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)

	to := []string{config.EmailSenderAddress}
	subject := "A test email"
	content := `
	<h1>Hello World</h1>
	<p>This is a test message from Simple Bank</p>
	`
	attachFiles := []string{"../README.md"}

	err = sender.SendEmail(subject, content, to, nil, nil, attachFiles)
	require.NoError(t, err)
}
