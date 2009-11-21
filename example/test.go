package main

import (
	"irc";
	"fmt";
	"os";
)

func main() {
	events := make(chan *irc.IRCEvent, 100);
	irccon, err := irc.IRC("irc.efnet.net:6667", "testgo", "testgo", events);
	if err != nil {
		fmt.Printf("%s\n", err);
		fmt.Printf("%#v\n", irccon);
		os.Exit(1);
	}
	for {
		event := <-events;
/*		switch event.Code {
	case UNKNOWN:
			fmt.Printf("%#v\n", event)
		case 0:
			fmt.Printf("%#v\n", event)
		case IRC_PRIVMSG:
			fmt.Printf("%#v\n", event)
		case IRC_CHAN_TOPIC:
			fmt.Printf("%#v\n", event)
		case IRC_CHAN_MODE:
			fmt.Printf("%#v\n", event)
		case IRC_ACTION:
			fmt.Printf("%#v\n", event)
		case IRC_WELCOME:
			irc.Join("#ggpre")
		}*/
		if event.Code == irc.IRC_WELCOME {
			irccon.Join("#gotestchan")
		} else if event.Code == irc.IRC_PRIVMSG {
			if event.Message == "!test" {
				irccon.Privmsg(event.Target, "Whatever man!");
			}
		}
		fmt.Printf("%#v\n", event);
	}
}
