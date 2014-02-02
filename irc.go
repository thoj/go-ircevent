// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const (
	VERSION = "go-ircevent v2.1"
)

func (irc *Connection) readLoop() {
	br := bufio.NewReaderSize(irc.socket, 512)

	for {
		msg, err := br.ReadString('\n')
		if err != nil {
			irc.Error <- err
			break
		}

		irc.lastMessage = time.Now()
		msg = msg[:len(msg)-2] //Remove \r\n
		event := &Event{Raw: msg}
		if msg[0] == ':' {
			if i := strings.Index(msg, " "); i > -1 {
				event.Source = msg[1:i]
				msg = msg[i+1 : len(msg)]

			} else {
				irc.log.Printf("Misformed msg from server: %#s\n", msg)
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
		/* XXX: len(args) == 0: args should be empty */

		irc.RunCallbacks(event)
	}

	irc.readerExit <- true
}

func (irc *Connection) writeLoop() {
	for {
		b, ok := <-irc.pwrite
		if !ok || b == "" || irc.socket == nil {
			break
		}

		if irc.Debug {
			irc.log.Printf("--> %s\n", b)
		}
		_, err := irc.socket.Write([]byte(b))
		if err != nil {
			irc.Error <- err
			break
		}
	}
	irc.writerExit <- true
}

//Pings the server if we have not recived any messages for 5 minutes
func (irc *Connection) pingLoop() {
	ticker := time.NewTicker(1 * time.Minute)   //Tick every minute.
	ticker2 := time.NewTicker(15 * time.Minute) //Tick every 15 minutes.
	for {
		select {
		case <-ticker.C:
			//Ping if we haven't received anything from the server within 4 minutes
			if time.Since(irc.lastMessage) >= (4 * time.Minute) {
				irc.SendRawf("PING %d", time.Now().UnixNano())
			}
		case <-ticker2.C:
			//Ping every 15 minutes.
			irc.SendRawf("PING %d", time.Now().UnixNano())
			//Try to recapture nickname if it's not as configured.
			if irc.nick != irc.nickcurrent {
				irc.nickcurrent = irc.nick
				irc.SendRawf("NICK %s", irc.nick)
			}
		case <-irc.endping:
			ticker.Stop()
			ticker2.Stop()
			irc.pingerExit <- true
			return
		}
	}
}

func (irc *Connection) Loop() {
	for !irc.stopped {
		err := <-irc.Error
		if irc.stopped {
			break
		}
		irc.log.Printf("Error: %s\n", err)
		irc.Disconnect()
		for !irc.stopped {
			if err = irc.Connect(irc.server); err != nil {
				irc.log.Printf("Error: %s\n", err)
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}
	}
}

func (irc *Connection) Quit() {
	irc.SendRaw("QUIT")
	irc.stopped = true
	irc.Disconnect()
}

func (irc *Connection) Join(channel string) {
	irc.pwrite <- fmt.Sprintf("JOIN %s\r\n", channel)
}

func (irc *Connection) Part(channel string) {
	irc.pwrite <- fmt.Sprintf("PART %s\r\n", channel)
}

func (irc *Connection) Notice(target, message string) {
	irc.pwrite <- fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}

func (irc *Connection) Noticef(target, format string, a ...interface{}) {
	irc.Notice(target, fmt.Sprintf(format, a...))
}

func (irc *Connection) Privmsg(target, message string) {
	irc.pwrite <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}

func (irc *Connection) Privmsgf(target, format string, a ...interface{}) {
	irc.Privmsg(target, fmt.Sprintf(format, a...))
}

func (irc *Connection) SendRaw(message string) {
	irc.pwrite <- message + "\r\n"
}

func (irc *Connection) SendRawf(format string, a ...interface{}) {
	irc.SendRaw(fmt.Sprintf(format, a...))
}

func (irc *Connection) Nick(n string) {
	irc.nick = n
	irc.SendRawf("NICK %s", n)
}

func (irc *Connection) GetNick() string {
	return irc.nickcurrent
}

// Sends all buffered messages (if possible),
// stops all goroutines and then closes the socket.
func (irc *Connection) Disconnect() {
	close(irc.pwrite)
	close(irc.pread)
	irc.endping <- true

	<-irc.readerExit
	<-irc.writerExit
	<-irc.pingerExit
	irc.socket.Close()
	irc.socket = nil
}

func (irc *Connection) Reconnect() error {
	return irc.Connect(irc.server)
}

func (irc *Connection) Connect(server string) error {
	irc.server = server
	irc.stopped = false

	var err error
	if irc.UseTLS {
		irc.socket, err = tls.Dial("tcp", irc.server, irc.TLSConfig)
	} else {
		irc.socket, err = net.Dial("tcp", irc.server)
	}
	if err != nil {
		return err
	}
	irc.log.Printf("Connected to %s (%s)\n", irc.server, irc.socket.RemoteAddr())

	irc.pread = make(chan string, 10)
	irc.pwrite = make(chan string, 10)
	irc.Error = make(chan error, 2)

	go irc.readLoop()
	go irc.writeLoop()
	go irc.pingLoop()

	if len(irc.Password) > 0 {
		irc.pwrite <- fmt.Sprintf("PASS %s\r\n", irc.Password)
	}
	irc.pwrite <- fmt.Sprintf("NICK %s\r\n", irc.nick)
	irc.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", irc.user, irc.user)
	return nil
}

func IRC(nick, user string) *Connection {
	irc := &Connection{
		nick:       nick,
		user:       user,
		log:        log.New(os.Stdout, "", log.LstdFlags),
		readerExit: make(chan bool),
		writerExit: make(chan bool),
		pingerExit: make(chan bool),
		endping:    make(chan bool),
	}
	irc.setupCallbacks()
	return irc
}
