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
* PRIVMSG
* MODE
* JOIN

+Many more


AddCallback Example
---------
	ircobj.AddCallback("PRIVMSG", func(event *irc.Event) {
		//e.Message contains the message
		//e.Nick Contains the sender
		//e.Arguments[0] Contains the channel
	});

Commands
--------
	irc.IRC("<nick>", "<user>") //Create new ircobj
	ircobj.Password = "[server password]"
	ircobj.Connect("irc.someserver.com:6667") //Connect to server
	ircobj.Sendraw("<string>") //sends string to server. Adds \r\n
	ircobj.Join("#channel [password]") 
	ircobj.Privmsg("#channel", "msg")
	ircobj.Privmsg("nickname", "msg")
	ircobj.Notice("<nickname | #channel>", "msg")
