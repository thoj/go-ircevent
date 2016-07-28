package irc

import (
	"regexp"
	"strings"
)

//Struct to store Channel Info
type Channel struct {
	Topic string
	Mode  string
	Users map[string]User
}

type User struct {
	Host string
	Mode string
}

var mode_split = regexp.MustCompile("([%@+]{0,1})(.+)") //Half-Op, //Op, //Voice

func (irc *Connection) SetupNickTrack() {
	// 353: RPL_NAMEREPLY per RFC1459
	// will typically receive this on channel joins and when NAMES is
	// called via GetNicksOnCHan
	irc.AddCallback("353", func(e *Event) {
		// get chan
		channelName := e.Arguments[2]
		// check if chan exists in map
		_, ok := irc.Channels[channelName]

		// if not make one
		if ok != true {
			irc.Channels[channelName] = Channel{Users: make(map[string]User)}
		}
		// split the datat into a slice
		for _, modenick := range strings.Split(e.Message(), " ") {
			nickandmode := mode_split.FindStringSubmatch(modenick)
			u := User{}
			if len(nickandmode) == 3 {
				if nickandmode[1] == "@" {
					u.Mode = "+o" // Ooof should be mode struct?
				} else if nickandmode[1] == "+" {
					u.Mode = "+v" // Ooof should be mode struct?
				} else if nickandmode[1] == "%" {
					u.Mode = "+h"
				}
				irc.Channels[channelName].Users[nickandmode[2]] = u
			} else {
				irc.Channels[channelName].Users[modenick] = u
			}
		}
	})

	irc.AddCallback("MODE", func(e *Event) {
		channelName := e.Arguments[0]
		if len(e.Arguments) == 3 { // 3 == for channel 2 == for user on server
			if _, ok := irc.Channels[channelName]; ok != true {
				irc.Channels[channelName] = Channel{Users: make(map[string]User)}
			}
			if _, ok := irc.Channels[channelName].Users[e.Arguments[2]]; ok != true {
				irc.Channels[channelName].Users[e.Arguments[2]] = User{Mode: e.Arguments[1]}
			} else {
				u := irc.Channels[channelName].Users[e.Arguments[2]]
				u.Mode = e.Arguments[1]
				irc.Channels[channelName].Users[e.Arguments[2]] = u
			}
		}
	})

	//Really hacky since the message from the server does not include the channel
	irc.AddCallback("NICK", func(e *Event) {
		if len(e.Arguments) == 1 { // Sanity check
			for k, _ := range irc.Channels {
				if _, ok := irc.Channels[k].Users[e.Nick]; ok {
					u := irc.Channels[k].Users[e.Nick]
					u.Host = e.Host
					irc.Channels[k].Users[e.Arguments[0]] = u //New nick
					delete(irc.Channels[k].Users, e.Nick)     //Delete old
				}
			}
		}
	})

	irc.AddCallback("JOIN", func(e *Event) {
		channelName := e.Arguments[0]
		if _, ok := irc.Channels[channelName]; ok != true {
			irc.Channels[channelName] = Channel{Users: make(map[string]User)}
		}
		irc.Channels[channelName].Users[e.Nick] = User{Host: e.Source}
	})

	irc.AddCallback("PART", func(e *Event) {
		channelName := e.Arguments[0]
		delete(irc.Channels[channelName].Users, e.Nick)
	})

	irc.AddCallback("QUIT", func(e *Event) {
		for k, _ := range irc.Channels {
			delete(irc.Channels[k].Users, e.Nick)
		}
	})
}
