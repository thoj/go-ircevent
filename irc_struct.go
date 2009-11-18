package irc

import (
	"os";
	"net";
)

type IRCEventCode int
const (
	IRC_NOTICE_AUTH	IRCEventCode	= 1 << iota;
	IRC_PING;
	IRC_QUIT;
	IRC_WELCOME;
	IRC_SERVER_INFO;
	IRC_SERVER_UPTIME;
	IRC_SERVER_VERSION;
	IRC_START_MOTD;
	IRC_MOTD;
	IRC_END_MOTD;
	IRC_CHANINFO;

	IRC_STAT_USERS;
	IRC_STAT_OPERS;
	IRC_STAT_UNKN;
	IRC_STAT_CONNS;

	IRC_CHAN_TIMESTAMP;
	IRC_CHAN_NICKLIST;
	IRC_CHAN_TOPIC;
	IRC_CHAN_MODE;

	IRC_PRIVMSG;
	IRC_ACTION;
	IRC_JOIN;

	IRC_MODE;

	ERROR;
	UNKNOWN;
)

type IRCConnection struct {
	socket		net.Conn;
	pread, pwrite	chan string;
	perror		chan os.Error;
	EventChan	chan *IRCEvent;
	Error		os.Error;
	nick	string;
	user	string;
	registered	bool;
}

type IRCEvent struct {
	Message		string;
	RawMessage	string;
	Sender		string;
	SenderHost	string;
	SenderUser	string;
	Target		string;
	Channel		string;
	Code		IRCEventCode;
	Error		os.Error;
}
