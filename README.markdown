Description
----------

Event based irc client library.


Features
---------
* Event based. Register Callbacks for the events you need to handle.
* Handles basic irc demands for you:
** Standard CTCP
** Reconnections on errors
** Detect stoned servers

Install
----------
	$ git clone git@github.com:thoj/Go-IRC-Client-Library.git
	$ cd Go-IRC-Client-Library
	$ make 
	$ make install

Example
----------
See example/test.go

Events for callbacks
---------
*001 Welcome
*PING
*CTCP Unknown CTCP
*CTCP_VERSION Version request (Handled internaly)
*CTCP_USERINFO
*CTCP_CLIENTINFO
*CTCP_TIME
*CTCP_PING
*PRIVMSG
*MODE
*JOIN

+Many more


AddCallback Example
---------
	ircobj.AddCallback("PRIVMSG", func(event *irc.IRCEvent) {
		//e.Message contains the message
		//e.Nick Contains the sender
		//e.Arguments[0] Contains the channel
	});

Commands
--------
	ircobj.Sendraw("<string>") //sends string to server. Adds \r\n
	ircobj.Join("#channel [password]") 
	ircobj.Privmsg("#channel", "msg")
	ircobj.Privmsg("nickname", "msg")
	ircobj.Notice("nickname or #channel", "msg")
