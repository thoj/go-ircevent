Description
----------

Event based irc client library.


Features
---------
* Event based. Register Callbacks for the events you need to handle.
* Handles basic irc demands for you
	* Standard CTCP
	* Reconnections on errors
	* Detect stoned servers

Install
----------
	$ go get github.com/thoj/go-ircevent

Example
----------
See test/irc_test.go

Events for callbacks
---------
* 001 Welcome
* PING
* CTCP Unknown CTCP
* CTCP_VERSION Version request (Handled internaly)
* CTCP_USERINFO
* CTCP_CLIENTINFO
* CTCP_TIME
* CTCP_PING
* CTCP_ACTION (/me)
* PRIVMSG
* MODE
* JOIN

+Many more

AddCallback Example
---------
	ircobj.AddCallback("PRIVMSG", func(event *irc.Event) {
		//e.Message() contains the message
		//e.Nick Contains the sender
		//e.Arguments[0] Contains the channel
	});

Commands
--------
	ircobj := irc.IRC("<nick>", "<user>") //Create new ircobj
	//Set options
	ircobj.UseTLS = true //default is false
	//ircobj.TLSOptions //set ssl options
	ircobj.Password = "[server password]"
	//Commands
	ircobj.Connect("irc.someserver.com:6667") //Connect to server
	ircobj.Sendraw("<string>") //sends string to server. Adds \r\n
	ircobj.Sendrawf("<formatstring>", ...) //sends formatted string to server.n
	ircobj.Join("<#channel> [password]") 
	ircobj.Nick("newnick") 
	ircobj.Privmsg("<nickname | #channel>", "msg")
	ircobj.Privmsgf(<nickname | #channel>, "<formatstring>", ...)
	ircobj.Notice("<nickname | #channel>", "msg")
	ircobj.Noticef("<nickname | #channel>", "<formatstring>", ...)

Note
---------
Events have recently been updated so there is no longer a message field
because the message is technically another argument. This may break some
systems, as there will be one more argument in stead of a Message.
It's also worth noting that Event.Message() is a convenience function
that will now grab the last argument, or return an empty string if there isn't one.

There is currently a workaround in place if you don't want to make the changes right now.
Simply set `conn.OldSplitStyle = true` and replace `event.Message` with `event.Message()`
This will revert to the old style of message splitting, where everything after the : is a Message
