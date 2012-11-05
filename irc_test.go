package irc

import (
	//	irc "github.com/thoj/Go-IRC-Client-Library"
	"fmt"
	"testing"
)


func TestConnection(t *testing.T) {
	irccon := IRC("invisible", "invisible")

	fmt.Printf("Testing connection\n")

	err := irccon.Connect("irc.freenode.net:6667")

	fmt.Printf("Connecting...")

	if err != nil {
		t.Fatal("Can't connect to freenode.")
	}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#invisible") })

	irccon.AddCallback("PRIVMSG" , func(e *Event) {
		irccon.Privmsg("#invisible", "WHAT IS THIS\n")
		fmt.Printf("Got private message, likely should respond!\n")
		irccon.Privmsg(e.Nick , "WHAT")


	})

	irccon.Loop()


}
