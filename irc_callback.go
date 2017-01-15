package irc

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var NickChanMap = map[string]chan []string{}
var NickChanState = map[string][]string{}

// Register a callback to a connection and event code. A callback is a function
// which takes only an Event pointer as parameter. Valid event codes are all
// IRC/CTCP commands and error/response codes. This function returns the ID of
// the registered callback for later management.
func (irc *Connection) AddCallback(eventcode string, callback func(*Event)) int {
	eventcode = strings.ToUpper(eventcode)
	id := 0
	if _, ok := irc.events[eventcode]; !ok {
		irc.events[eventcode] = make(map[int]func(*Event))
		id = 0
	} else {
		id = len(irc.events[eventcode])
	}
	irc.events[eventcode][id] = callback
	return id
}

// Remove callback i (ID) from the given event code. This functions returns
// true upon success, false if any error occurs.
func (irc *Connection) RemoveCallback(eventcode string, i int) bool {
	eventcode = strings.ToUpper(eventcode)

	if event, ok := irc.events[eventcode]; ok {
		if _, ok := event[i]; ok {
			delete(irc.events[eventcode], i)
			return true
		}
		irc.Log.Printf("Event found, but no callback found at id %d\n", i)
		return false
	}

	irc.Log.Println("Event not found")
	return false
}

// Remove all callbacks from a given event code. It returns true
// if given event code is found and cleared.
func (irc *Connection) ClearCallback(eventcode string) bool {
	eventcode = strings.ToUpper(eventcode)

	if _, ok := irc.events[eventcode]; ok {
		irc.events[eventcode] = make(map[int]func(*Event))
		return true
	}

	irc.Log.Println("Event not found")
	return false
}

// Replace callback i (ID) associated with a given event code with a new callback function.
func (irc *Connection) ReplaceCallback(eventcode string, i int, callback func(*Event)) {
	eventcode = strings.ToUpper(eventcode)

	if event, ok := irc.events[eventcode]; ok {
		if _, ok := event[i]; ok {
			event[i] = callback
			return
		}
		irc.Log.Printf("Event found, but no callback found at id %d\n", i)
	}
	irc.Log.Printf("Event not found. Use AddCallBack\n")
}

// Execute all callbacks associated with a given event.
func (irc *Connection) RunCallbacks(event *Event) {
	msg := event.Message()
	if event.Code == "PRIVMSG" && len(msg) > 2 && msg[0] == '\x01' {
		event.Code = "CTCP" //Unknown CTCP

		if i := strings.LastIndex(msg, "\x01"); i > 0 {
			msg = msg[1:i]
		} else {
			irc.Log.Printf("Invalid CTCP Message: %s\n", strconv.Quote(msg))
			return
		}

		if msg == "VERSION" {
			event.Code = "CTCP_VERSION"

		} else if msg == "TIME" {
			event.Code = "CTCP_TIME"

		} else if strings.HasPrefix(msg, "PING") {
			event.Code = "CTCP_PING"

		} else if msg == "USERINFO" {
			event.Code = "CTCP_USERINFO"

		} else if msg == "CLIENTINFO" {
			event.Code = "CTCP_CLIENTINFO"

		} else if strings.HasPrefix(msg, "ACTION") {
			event.Code = "CTCP_ACTION"
			if len(msg) > 6 {
				msg = msg[7:]
			} else {
				msg = ""
			}
		}

		event.Arguments[len(event.Arguments)-1] = msg
	}

	if callbacks, ok := irc.events[event.Code]; ok {
		if irc.VerboseCallbackHandler {
			irc.Log.Printf("%v (%v) >> %#v\n", event.Code, len(callbacks), event)
		}

		for _, callback := range callbacks {
			callback(event)
		}
	} else if irc.VerboseCallbackHandler {
		irc.Log.Printf("%v (0) >> %#v\n", event.Code, event)
	}

	if callbacks, ok := irc.events["*"]; ok {
		if irc.VerboseCallbackHandler {
			irc.Log.Printf("%v (0) >> %#v\n", event.Code, event)
		}

		for _, callback := range callbacks {
			callback(event)
		}
	}
}

// Set up some initial callbacks to handle the IRC/CTCP protocol.
func (irc *Connection) setupCallbacks() {
	irc.events = make(map[string]map[int]func(*Event))

	//Handle error events.
	irc.AddCallback("ERROR", func(e *Event) { irc.Disconnect() })

	//Handle ping events
	irc.AddCallback("PING", func(e *Event) { irc.SendRaw("PONG :" + e.Message()) })

	//Version handler
	irc.AddCallback("CTCP_VERSION", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01VERSION %s\x01", e.Nick, irc.Version)
	})

	irc.AddCallback("CTCP_USERINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01USERINFO %s\x01", e.Nick, irc.user)
	})

	irc.AddCallback("CTCP_CLIENTINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01CLIENTINFO PING VERSION TIME USERINFO CLIENTINFO\x01", e.Nick)
	})

	irc.AddCallback("CTCP_TIME", func(e *Event) {
		ltime := time.Now()
		irc.SendRawf("NOTICE %s :\x01TIME %s\x01", e.Nick, ltime.String())
	})

	irc.AddCallback("CTCP_PING", func(e *Event) { irc.SendRawf("NOTICE %s :\x01%s\x01", e.Nick, e.Message()) })

	// 437: ERR_UNAVAILRESOURCE "<nick/channel> :Nick/channel is temporarily unavailable"
	// Add a _ to current nick. If irc.nickcurrent is empty this cannot
	// work. It has to be set somewhere first in case the nick is already
	// taken or unavailable from the beginning.
	irc.AddCallback("437", func(e *Event) {
		// If irc.nickcurrent hasn't been set yet, set to irc.nick
		if irc.nickcurrent == "" {
			irc.nickcurrent = irc.nick
		}

		if len(irc.nickcurrent) > 8 {
			irc.nickcurrent = "_" + irc.nickcurrent
		} else {
			irc.nickcurrent = irc.nickcurrent + "_"
		}
		irc.SendRawf("NICK %s", irc.nickcurrent)
	})

	// 433: ERR_NICKNAMEINUSE "<nick> :Nickname is already in use"
	// Add a _ to current nick.
	irc.AddCallback("433", func(e *Event) {
		// If irc.nickcurrent hasn't been set yet, set to irc.nick
		if irc.nickcurrent == "" {
			irc.nickcurrent = irc.nick
		}

		if len(irc.nickcurrent) > 8 {
			irc.nickcurrent = "_" + irc.nickcurrent
		} else {
			irc.nickcurrent = irc.nickcurrent + "_"
		}
		irc.SendRawf("NICK %s", irc.nickcurrent)
	})

	irc.AddCallback("PONG", func(e *Event) {
		ns, _ := strconv.ParseInt(e.Message(), 10, 64)
		delta := time.Duration(time.Now().UnixNano() - ns)
		if irc.Debug {
			irc.Log.Printf("Lag: %.3f s\n", delta.Seconds())
		}
	})

	// NICK Define a nickname.
	// Set irc.nickcurrent to the new nick actually used in this connection.
	irc.AddCallback("NICK", func(e *Event) {
		if e.Nick == irc.nick {
			irc.nickcurrent = e.Message()
		}
	})

	// 1: RPL_WELCOME "Welcome to the Internet Relay Network <nick>!<user>@<host>"
	// Set irc.nickcurrent to the actually used nick in this connection.
	irc.AddCallback("001", func(e *Event) {
		irc.Lock()
		irc.nickcurrent = e.Arguments[0]
		irc.Unlock()
	})

	// 353: RPL_NAMEREPLY per RFC1459
	// will typically receive this on channel joins and when NAMES is
	// called via GetNicksOnCHan
	irc.AddCallback("353", func(e *Event) {
		// get chan
		re := regexp.MustCompile(`#\S+\s`)
		channelName := re.FindString(e.Raw)
		// check if chan exists in map
		_, ok := NickChanMap[channelName]

		// if not make one
		if ok != true {
			ch := make(chan []string, 1000)
			NickChanMap[channelName] = ch
		}
		// split the datat into a slice
		data := strings.Split(e.Message(), " ")

		// then send
		NickChanMap[channelName] <- data
	})

	// 363: RPL_ENDOFNAMES per RFC1459
	// will receive this so we know that no more 353s are coming
	irc.AddCallback("366", func(e *Event) {
		go func(e *Event) {
			// get chan
			re := regexp.MustCompile(`#\S+\s`)
			channelName := re.FindString(e.Raw)
			// if channel doesn't exist return
			c, ok := NickChanMap[channelName]
			if ok != true {
				return
			}
			// Note, callback 353 is not in a goroutine on purpose
			// to prevent chance of channel being closed
			// prematurely
			close(c) // there's probably a more elegant way to do this
			// append list of names
			var theList []string
			for r := range c {
				theList = append(theList, r...)
			}
			// reset the channel with a new one since we closed the other
			// one
			ch := make(chan []string, 1000)
			NickChanMap[channelName] = ch
			// add theList of nicks to a global state var
			NickChanState[channelName] = theList

		}(e)
	})

}
