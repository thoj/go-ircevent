// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"fmt"
	"net"
	"os"
	"bufio"
	"strings"
	"time"
)

const (
	VERSION = "GolangBOT v1.0"
)

var error bool

func reader(irc *IRCConnection) {
	br := bufio.NewReader(irc.socket)
	for !error {
		msg, err := br.ReadString('\n')
		if err != nil {
			irc.Error <- err
			break
		}
		irc.lastMessage = time.Seconds()
		msg = msg[0 : len(msg)-2] //Remove \r\n
		event := &IRCEvent{Raw: msg}
		if msg[0] == ':' {
			if i := strings.Index(msg, " "); i > -1 {
				event.Source = msg[1:i]
				msg = msg[i+1 : len(msg)]
			} else {
				fmt.Printf("Misformed msg from server: %#s\n", msg)
			}
			if i, j := strings.Index(event.Source, "!"), strings.Index(event.Source, "@"); i > -1 && j > -1 {
				event.Nick = event.Source[0:i]
				event.User = event.Source[i+1 : j]
				event.Host = event.Source[j+1 : len(event.Source)]
			}
		}
		args := strings.SplitN(msg, " :", 2)
		if len(args) > 1 {
			event.Message = args[1]
		}
		args = strings.Split(args[0], " ")
		event.Code = strings.ToUpper(args[0])
		if len(args) > 1 {
			event.Arguments = args[1:len(args)]
		}
		irc.RunCallbacks(event)
	}
	irc.syncreader <- true
}

func writer(irc *IRCConnection) {
	b, ok := <-irc.pwrite
	for !error && ok {
		if b == "" || irc.socket == nil {
			break
		}
		_, err := irc.socket.Write([]byte(b))
		if err != nil {
			fmt.Printf("%s\n", err)
			irc.Error <- err
			break
		}
		b, ok = <-irc.pwrite
	}
	irc.syncwriter <- true
}

//Pings the server if we have not recived any messages for 5 minutes
func pinger(i *IRCConnection) {
	i.ticker = time.Tick(1000 * 1000 * 1000 * 60 * 1)   //Tick every minute.
	i.ticker2 = time.Tick(1000 * 1000 * 1000 * 60 * 15) //Tick every 15 minutes.
	for {
		select {
		case <-i.ticker:
			//Ping if we haven't recived anything from the server within 4 minutes
			if time.Seconds()-i.lastMessage >= 60*4 {
				i.SendRaw(fmt.Sprintf("PING %d", time.Nanoseconds()))
			}
		case <-i.ticker2:
			//Ping every 15 minutes.
			i.SendRaw(fmt.Sprintf("PING %d", time.Nanoseconds()))
			//Try to recapture nickname if it's not as configured.
			if i.nick != i.nickcurrent {
				i.nickcurrent = i.nick
				i.SendRaw(fmt.Sprintf("NICK %s", i.nick))
			}
		}
	}
}

func (irc *IRCConnection) Cycle() {
	irc.SendRaw("QUIT")
	irc.Reconnect()
}

func (irc *IRCConnection) Quit() {
	irc.quitting = true
	irc.SendRaw("QUIT")
}

func (irc *IRCConnection) Join(channel string) {
	irc.pwrite <- fmt.Sprintf("JOIN %s\r\n", channel)
}

func (irc *IRCConnection) Part(channel string) {
	irc.pwrite <- fmt.Sprintf("PART %s\r\n", channel)
}

func (irc *IRCConnection) Notice(target, message string) {
	irc.pwrite <- fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}

func (irc *IRCConnection) Privmsg(target, message string) {
	irc.pwrite <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}

func (irc *IRCConnection) SendRaw(message string) {
	fmt.Printf("--> %s\n", message)
	irc.pwrite <- fmt.Sprintf("%s\r\n", message)
}

func (i *IRCConnection) Reconnect() os.Error {
	close(i.pwrite)
	close(i.pread)
	<-i.syncreader
	<-i.syncwriter
	for {
		fmt.Printf("Reconnecting to %s\n", i.server)
		var err os.Error
		i.socket, err = net.Dial("tcp", i.server)
		if err == nil {
			break
		}
		fmt.Printf("Error: %s\n", err)
	}
	error = false
	fmt.Printf("Connected to %s (%s)\n", i.server, i.socket.RemoteAddr())
	go reader(i)
	go writer(i)
	i.pwrite <- fmt.Sprintf("NICK %s\r\n", i.nick)
	i.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", i.user, i.user)
	return nil
}

func (i *IRCConnection) Loop() {
	for !i.quitting {
		e := <-i.Error
		if i.quitting {
			break
		}
		fmt.Printf("Error: %s\n", e)
		error = true
		i.Reconnect()
	}
	close(i.pwrite)
	close(i.pread)
	<-i.syncreader
	<-i.syncwriter
}

func (i *IRCConnection) Connect(server string) os.Error {
	i.server = server
	fmt.Printf("Connecting to %s\n", i.server)
	var err os.Error
	i.socket, err = net.Dial("tcp", i.server)
	if err != nil {
		return err
	}
	fmt.Printf("Connected to %s (%s)\n", i.server, i.socket.RemoteAddr())
	i.pread = make(chan string, 100)
	i.pwrite = make(chan string, 100)
	i.Error = make(chan os.Error, 10)
	i.syncreader = make(chan bool)
	i.syncwriter = make(chan bool)
	go reader(i)
	go writer(i)
	go pinger(i)
	i.pwrite <- fmt.Sprintf("NICK %s\r\n", i.nick)
	i.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", i.user, i.user)
	if len(i.Password) > 0 {
		i.pwrite <- fmt.Sprintf("PASS %s\r\n", i.Password)
	}
	return nil
}

func IRC(nick, user string) *IRCConnection {
	irc := new(IRCConnection)
	irc.registered = false
	irc.pread = make(chan string, 100)
	irc.pwrite = make(chan string, 100)
	irc.Error = make(chan os.Error)
	irc.nick = nick
	irc.user = user
	irc.VerboseCallbackHandler = true
	irc.setupCallbacks()
	return irc
}
