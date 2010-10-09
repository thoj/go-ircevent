package main

import (
	//	irc "github.com/thoj/Go-IRC-Client-Library"
	"fmt"
	"os"
	"irc"
)

func main() {
	irccon := irc.IRC("testgo", "testgo")
	err := irccon.Connect("irc.efnet.net:6667")
	if err != nil {
		fmt.Printf("%s\n", err)
		fmt.Printf("%#v\n", irccon)
		os.Exit(1)
	}
	irccon.AddCallback("001", func(e *irc.IRCEvent) { irccon.Join("#testgo1") })
	irccon.AddCallback("001", func(e *irc.IRCEvent) { irccon.Join("#testgo2") })
	irccon.AddCallback("001", func(e *irc.IRCEvent) { irccon.Join("#testgo3") })
	irccon.AddCallback("001", func(e *irc.IRCEvent) { irccon.Join("#testgo4") })
	irccon.AddCallback("001", func(e *irc.IRCEvent) { irccon.Join("#testgo5") })
	irccon.AddCallback("001", func(e *irc.IRCEvent) { irccon.Join("#testgo6") })
	irccon.ReplaceCallback("001", 0, func(e *irc.IRCEvent) { irccon.Join("#testgo01") })
	irccon.ReplaceCallback("001", 1, func(e *irc.IRCEvent) { irccon.Join("#testgo02") })
	irccon.ReplaceCallback("001", 2, func(e *irc.IRCEvent) { irccon.Join("#testgo03") })
	irccon.ReplaceCallback("001", 3, func(e *irc.IRCEvent) { irccon.Join("#testgo04") })
	irccon.ReplaceCallback("001", 4, func(e *irc.IRCEvent) { irccon.Join("#testgo05") })
	irccon.ReplaceCallback("001", 6, func(e *irc.IRCEvent) { irccon.Join("#testgo06") })
	irccon.Loop()
}
