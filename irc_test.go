package irc

import (
//	"github.com/thoj/go-ircevent"
	"testing"
)


func TestConnection(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	err := irccon.Connect("irc.freenode.net:6667")
	if err != nil {
		t.Fatal("Can't connect to freenode.")
	}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#go-eventirc") })

	irccon.AddCallback("366" , func(e *Event) {
		irccon.Privmsg("#go-eventirc", "Test Message\n")
		irccon.Quit();
	})

	irccon.Loop()
}

func TestConnectionSSL(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	irccon.UseTLS = true
	err := irccon.Connect("irc.freenode.net:7000")
	if err != nil {
		t.Fatal("Can't connect to freenode.")
	}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#go-eventirc") })

	irccon.AddCallback("366" , func(e *Event) {
		irccon.Privmsg("#go-eventirc", "Test Message\n")
		irccon.Quit();
	})

	irccon.Loop()
}
