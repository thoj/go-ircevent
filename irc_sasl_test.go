package irc

import (
	"crypto/tls"
	"os"
	"testing"
	"time"
)

// set SASLLogin and SASLPassword environment variables before testing
func TestConnectionSASL(t *testing.T) {
	SASLServer := "irc.freenode.net:7000"
	SASLLogin := os.Getenv("SASLLogin")
	SASLPassword := os.Getenv("SASLPassword")

	if SASLLogin == "" {
		t.Skip("Define SASLLogin and SASLPasword environment varables to test SASL")
	}
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	irccon.Debug = true
	irccon.UseTLS = true
	irccon.UseSASL = true
	irccon.SASLLogin = SASLLogin
	irccon.SASLPassword = SASLPassword
	irccon.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#go-eventirc") })

	irccon.AddCallback("366", func(e *Event) {
		irccon.Privmsg("#go-eventirc", "Test Message SASL\n")
		time.Sleep(2 * time.Second)
		irccon.Quit()
	})

	err := irccon.Connect(SASLServer)
	if err != nil {
		t.Fatal("SASL failed")
	}
	irccon.Loop()
}
