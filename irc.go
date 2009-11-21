// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"fmt";
	"net";
	"os";
	"bufio";
	"regexp";
	"strings";
)


func reader(irc *IRCConnection) {
	br := bufio.NewReader(irc.socket);
	for {
		msg, err := br.ReadString('\n');
		if err != nil {
			fmt.Printf("%s\n", err);
			irc.perror <- err;
		}
		irc.pread <- msg;
	}
}

func writer(irc *IRCConnection) {
	for {
		b := strings.Bytes(<-irc.pwrite);
		_, err := irc.socket.Write(b);
		if err != nil {
			fmt.Printf("%s\n", err);
			irc.perror <- err;
		}
	}
}

var rx_server_msg = regexp.MustCompile("^:([^ ]+) ([^ ]+) ([^ ]+) :(.*)\r\n")
var rx_server_msg_c = regexp.MustCompile("^:([^ ]+) ([^ ]+) ([^ ]+) [@]* ([^ ]+) :(.*)\r\n")
var rx_server_msg_p = regexp.MustCompile("^:([^ ]+) ([^ ]+) ([^ ]+) (.*)\r\n")
var rx_server_cmd = regexp.MustCompile("^([^:]+) :(.*)\r\n")	//AUTH NOTICE, PING, ERROR
var rx_user_action = regexp.MustCompile("^:([^!]+)!([^@]+)@([^ ]+) ([^ ]+) [:]*(.*)\r\n")
var rx_user_msg = regexp.MustCompile("^:([^!]+)!([^@]+)@([^ ]+) ([^ ]+) ([^ ]+) :(.*)\r\n")

func (irc *IRCConnection) handle_command(msg string) *IRCEvent {
	e := new(IRCEvent);
	e.RawMessage = msg;
	if matches := rx_user_msg.MatchStrings(msg); len(matches) == 7 {
		e.Sender = matches[1];
		e.SenderUser = matches[2];
		e.SenderHost = matches[3];
		e.Message = matches[6];
		e.Target = matches[5];
		switch matches[4] {
		case "PRIVMSG":
			e.Code = IRC_PRIVMSG
		case "ACTION":
			e.Code = IRC_ACTION
		}
		return e;
	} else if matches := rx_user_action.MatchStrings(msg); len(matches) == 6 {
		e.Sender = matches[1];
		e.SenderUser = matches[2];
		e.SenderHost = matches[3];
		e.Message = matches[5];
		e.Target = matches[5];
		e.Channel = matches[5];
		switch matches[4] {
		case "JOIN":
			e.Code = IRC_JOIN
		case "MODE":
			e.Code = IRC_CHAN_MODE
		}
		return e;
	} else if matches := rx_server_msg_c.MatchStrings(msg); len(matches) == 6 {
		e.Sender = matches[1];
		e.Target = matches[3];
		e.Channel = matches[4];
		e.Message = matches[5];
		switch matches[2] {
		case "366":
			e.Code = IRC_CHAN_NICKLIST
		case "332":
			e.Code = IRC_CHAN_TOPIC
		}
		return e;
	} else if matches := rx_server_msg.MatchStrings(msg); len(matches) == 5 {
		e.Sender = matches[1];
		e.Target = matches[3];
		e.Message = matches[4];
		switch matches[2] {
		case "001":
			e.Code = IRC_WELCOME
		case "002":
			e.Code = IRC_SERVER_INFO
		case "003":
			e.Code = IRC_SERVER_UPTIME
		case "250":
			e.Code = IRC_STAT_USERS
		case "251":
			e.Code = IRC_STAT_USERS
		case "255":
			e.Code = IRC_STAT_USERS
		case "372":
			e.Code = IRC_MOTD
		case "375":
			e.Code = IRC_START_MOTD
		case "376":
			e.Code = IRC_END_MOTD
		case "MODE":
			e.Code = IRC_MODE
		}
		return e;
	} else if matches := rx_server_msg_p.MatchStrings(msg); len(matches) == 5 {
		e.Sender = matches[1];
		e.Target = matches[3];
		e.Message = matches[4];
		switch matches[2] {
		case "252":
			e.Code = IRC_STAT_OPERS
		case "253":
			e.Code = IRC_STAT_UNKN
		case "254":
			e.Code = IRC_STAT_CONNS
		case "265":
			e.Code = IRC_STAT_USERS
		case "266":
			e.Code = IRC_STAT_USERS
		case "004":
			e.Code = IRC_SERVER_VERSION
		case "005":
			e.Code = IRC_CHANINFO
		case "332":
			e.Code = IRC_CHAN_TIMESTAMP
		case "353":
			e.Code = IRC_CHAN_NICKLIST
		}
		return e;
	} else if matches := rx_server_cmd.MatchStrings(msg); len(matches) == 3 {
		switch matches[1] {
		case "NOTICE AUTH":
			e.Code = IRC_NOTICE_AUTH;
			e.Message = matches[2];
		case "PING":
			e.Code = IRC_PING;
			e.Message = matches[2];
		case "ERROR":
			e.Code = IRC_PING;
			e.Message = matches[2];
			e.Error = os.ErrorString(matches[2]);
		}
		return e;
	}
	e.Message = msg;
	e.Code = UNKNOWN;
	return e;
}

func handler(irc *IRCConnection) {
	go reader(irc);
	go writer(irc);
	irc.pwrite <- fmt.Sprintf("NICK %s\r\n", irc.nick);
	irc.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :GolangBOT\r\n", irc.user);
	for {
		select {
		case msg := <-irc.pread:
			e := irc.handle_command(msg);
			switch e.Code {
			case IRC_PING:
				irc.pwrite <- fmt.Sprintf("PONG %s\r\n", e.Message)
			case IRC_PRIVMSG:
				if e.Message == "\x01VERSION\x01" {
					irc.pwrite <- fmt.Sprintf("NOTICE %s :\x01VERSION GolangBOT (tj)\x01\r\n", e.Sender)
				}
			}

			irc.EventChan <- e;
		case error := <-irc.perror:
			fmt.Printf("Piped error: %s\n", error);
			ee := new(IRCEvent);
			ee.Error = error;
			ee.Code = ERROR;
			irc.EventChan <- ee;
		}
	}
}

func (irc *IRCConnection) Join(channel string) {
	irc.pwrite <- fmt.Sprintf("JOIN %s\r\n", channel)
}

func (irc *IRCConnection) Notice(target, message string) {
	irc.pwrite <- fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}

func (irc *IRCConnection) Privmsg(target, message string) {
	irc.pwrite <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}

func IRC(server string, nick string, user string, events chan *IRCEvent) (*IRCConnection, os.Error) {
	irc := new(IRCConnection);

	irc.socket, irc.Error = net.Dial("tcp", "", server);
	if irc.Error != nil {
		return nil, irc.Error
	}
	irc.registered = false;
	irc.pread = make(chan string, 100);
	irc.pwrite = make(chan string, 100);
	irc.perror = make(chan os.Error, 10);
	irc.EventChan = events;
	irc.nick = nick;
	irc.user = user;
	go handler(irc);
	return irc, nil;
}
