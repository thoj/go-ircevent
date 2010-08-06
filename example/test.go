package main

import (
	irc "github.com/thoj/Go-IRC-Client-Library"
	"fmt"
	"os"
)

func main() {
	irccon := irc.IRC("testgo", "testgo")
	err := irccon.Connect("irc.efnet.net:6667")
	if err != nil {
		fmt.Printf("%s\n", err)
		fmt.Printf("%#v\n", irccon)
		os.Exit(1)
	}
	irccon.AddCallback("001", func(e *irc.IRCEvent) { irccon.Join("#testgo") })
	irccon.Loop();
}
