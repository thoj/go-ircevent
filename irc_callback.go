package irc

import (
	"strings"
	"fmt"
	"time"
	"strconv"
)


func (irc *IRCConnection) AddCallback(eventcode string, callback func(*IRCEvent)) {
	eventcode = strings.ToUpper(eventcode)
	if event, ok := irc.events[eventcode]; ok {
		// TODO: Grow this dynamically
		event = event[0 : len(event)+1]
		event[len(event)-1] = callback
	} else {
		event = make([]func(*IRCEvent), 1, 20)
		event[0] = callback
		irc.events[eventcode] = event
	}
}

func (irc *IRCConnection) ReplaceCallback(eventcode string, i uint8, callback func(*IRCEvent)) {
	eventcode = strings.ToUpper(eventcode)
	if event, ok := irc.events[eventcode]; ok {
		event[i] = callback
	} else {
		event = make([]func(*IRCEvent), 1, 20)
		event[0] = callback
		irc.events[eventcode] = event
	}
}

func (irc *IRCConnection) RunCallbacks(event *IRCEvent) {
	if event.Code == "PRIVMSG" && event.Message[0] == '\x01' {
		event.Code = "CTCP" //Unknown CTCP
		if i := strings.LastIndex(event.Message, "\x01"); i > -1 {
			event.Message = event.Message[1:i]
		}
		if event.Message == "VERSION" {
			event.Code = "CTCP_VERSION"
		} else if event.Message == "TIME" {
			event.Code = "CTCP_TIME"
		} else if event.Message[0:4] == "PING" {
			event.Code = "CTCP_PING"
		} else if event.Message == "USERINFO" {
			event.Code = "CTCP_USERINFO"
		} else if event.Message == "CLIENTINFO" {
			event.Code = "CTCP_CLIENTINFO"
		}
	}
	if callbacks, ok := irc.events[event.Code]; ok {
		for _, callback := range callbacks {
			go callback(event)
		}
	} else {
		fmt.Printf("No callback for: %#v\n", event)
	}
}

func (irc *IRCConnection) setupCallbacks() {
	irc.events = make(map[string][]func(*IRCEvent))

	//Handle ping events
	irc.AddCallback("PING", func(e *IRCEvent) { irc.SendRaw("PONG :" + e.Message) })

	//Version handler
	irc.AddCallback("CTCP_VERSION", func(e *IRCEvent) {
		irc.SendRaw(fmt.Sprintf("NOTICE %s :\x01VERSION %s\x01", e.Nick, VERSION))
	})

	irc.AddCallback("CTCP_USERINFO", func(e *IRCEvent) {
		irc.SendRaw(fmt.Sprintf("NOTICE %s :\x01USERINFO %s\x01", e.Nick, irc.user))
	})

	irc.AddCallback("CTCP_CLIENTINFO", func(e *IRCEvent) {
		irc.SendRaw(fmt.Sprintf("NOTICE %s :\x01CLIENTINFO PING VERSION TIME USERINFO CLIENTINFO\x01", e.Nick))
	})

	irc.AddCallback("CTCP_TIME", func(e *IRCEvent) {
		ltime := time.LocalTime();
		irc.SendRaw(fmt.Sprintf("NOTICE %s :\x01TIME %s\x01", e.Nick, ltime.String()))
	})

	irc.AddCallback("CTCP_PING", func(e *IRCEvent) { irc.SendRaw(fmt.Sprintf("NOTICE %s :\x01%s\x01", e.Nick, e.Message)) })

	irc.AddCallback("437", func(e *IRCEvent) {
		irc.nick = irc.nick + "_"
		irc.SendRaw(fmt.Sprintf("NICK %s", irc.nick))
	})

	irc.AddCallback("433", func(e *IRCEvent) {
		irc.nick = irc.nick + "_"
		irc.SendRaw(fmt.Sprintf("NICK %s", irc.nick))
	})

	irc.AddCallback("PONG", func(e *IRCEvent) {
		ns, _ := strconv.Atoi64(e.Message)
		fmt.Printf("Lag: %fs\n", float((time.Nanoseconds()-ns))/1000/1000/1000)
	})
}
