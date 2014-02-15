// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
This package provides an event based IRC client library. It allows to 
register callbacks for the events you need to handle. Its features 
include handling standard CTCP, reconnecting on errors and detecting 
stones servers.
Details of the IRC protocol can be found in the following RFCs:
https://tools.ietf.org/html/rfc1459
https://tools.ietf.org/html/rfc2810
https://tools.ietf.org/html/rfc2811
https://tools.ietf.org/html/rfc2812
https://tools.ietf.org/html/rfc2813
The details of the client-to-client protocol (CTCP) can be found here: http://www.irchelp.org/irchelp/rfc/ctcpspec.html
*/

package irc

import (
	"bufio"
	"crypto/tls"
	"errors"
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


// Read data from a connection. To be used as a goroutine.
func (irc *Connection) readLoop() {
	br := bufio.NewReaderSize(irc.socket, 512)

	for {
		select {
		case <-irc.endread:
			irc.readerExit <- true
			return
		default:
			// Set a read deadline based on the combined timeout and ping frequency
			// We should ALWAYS have received a response from the server within the timeout
			// after our own pings
			if irc.socket != nil {
				irc.socket.SetReadDeadline(time.Now().Add(irc.Timeout + irc.PingFreq))
			}

			msg, err := br.ReadString('\n')

			// We got past our blocking read, so bin timeout
			if irc.socket != nil {
				var zero time.Time
				irc.socket.SetReadDeadline(zero)
			}

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
					irc.Log.Printf("Misformed msg from server: %#s\n", msg)
				}

				if i, j := strings.Index(event.Source, "!"), strings.Index(event.Source, "@"); i > -1 && j > -1 {
					event.Nick = event.Source[0:i]
					event.User = event.Source[i+1 : j]
					event.Host = event.Source[j+1 : len(event.Source)]
				}
			}

			split := strings.SplitN(msg, " :", 2)
			args := strings.Split(split[0], " ")
			event.Code = strings.ToUpper(args[0])
			event.Arguments = args[1:]
			if len(split) > 1 {
				event.Arguments = append(event.Arguments, split[1])
			}

			/* XXX: len(args) == 0: args should be empty */

			irc.RunCallbacks(event)
		}
	}

	irc.readerExit <- true
}


// Loop to write to a connection. To be used as a goroutine.
func (irc *Connection) writeLoop() {
	for {
		select {
		case <-irc.endwrite:
			irc.writerExit <- true
			return
		default:
			b, ok := <-irc.pwrite
			if !ok || b == "" || irc.socket == nil {
				irc.writerExit <- true
				return
			}

			if irc.Debug {
				irc.Log.Printf("--> %s\n", b)
			}

			// Set a write deadline based on the time out
			irc.socket.SetWriteDeadline(time.Now().Add(irc.Timeout))

			_, err := irc.socket.Write([]byte(b))

			// Past blocking write, bin timeout
			var zero time.Time
			irc.socket.SetWriteDeadline(zero)

			if err != nil {
				irc.Error <- err
				irc.writerExit <- true
				return
			}
		}
	}
	irc.writerExit <- true
}


// Pings the server if we have not received any messages for 5 minutes
// to keep the connection alive. To be used as a goroutine.
func (irc *Connection) pingLoop() {
	ticker := time.NewTicker(1 * time.Minute) // Tick every minute for monitoring
	ticker2 := time.NewTicker(irc.PingFreq)   // Tick at the ping frequency.
	for {
		select {
		case <-ticker.C:
			//Ping if we haven't received anything from the server within the keep alive period
			if time.Since(irc.lastMessage) >= irc.KeepAlive {
				irc.SendRawf("PING %d", time.Now().UnixNano())
			}
		case <-ticker2.C:
			//Ping at the ping frequency
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


// Main loop to control the connection.
func (irc *Connection) Loop() {
	for !irc.stopped {
		err := <-irc.Error
		if irc.stopped {
			break
		}
		irc.Log.Printf("Error: %s\n", err)
		irc.Disconnect()
		for !irc.stopped {
			if err = irc.Connect(irc.server); err != nil {
				irc.Log.Printf("Error: %s\n", err)
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}
	}
}


// Quit the current connection and disconnect from the server
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.1.6
func (irc *Connection) Quit() {
	irc.SendRaw("QUIT")
	irc.stopped = true
	irc.Disconnect()
}


// Use the connection to join a given channel.
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.2.1
func (irc *Connection) Join(channel string) {
	irc.pwrite <- fmt.Sprintf("JOIN %s\r\n", channel)
}


// Leave a given channel.
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.2.2
func (irc *Connection) Part(channel string) {
	irc.pwrite <- fmt.Sprintf("PART %s\r\n", channel)
}


// Send a notification to a nickname. This is similar to Privmsg but must not receive replies.
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.4.2
func (irc *Connection) Notice(target, message string) {
	irc.pwrite <- fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}


// Send a formated notification to a nickname.
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.4.2
func (irc *Connection) Noticef(target, format string, a ...interface{}) {
	irc.Notice(target, fmt.Sprintf(format, a...))
}


// Send (private) message to a target (channel or nickname).
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.4.1
func (irc *Connection) Privmsg(target, message string) {
	irc.pwrite <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}


// Send formated string to specified target (channel or nickname).
func (irc *Connection) Privmsgf(target, format string, a ...interface{}) {
	irc.Privmsg(target, fmt.Sprintf(format, a...))
}


// Send raw string.
func (irc *Connection) SendRaw(message string) {
	irc.pwrite <- message + "\r\n"
}


// Send raw formated string.
func (irc *Connection) SendRawf(format string, a ...interface{}) {
	irc.SendRaw(fmt.Sprintf(format, a...))
}


// Set (new) nickname.
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.1.2
func (irc *Connection) Nick(n string) {
	irc.nick = n
	irc.SendRawf("NICK %s", n)
}


// Determine nick currently used with the connection.
func (irc *Connection) GetNick() string {
	return irc.nickcurrent
}


// Query information about a particular nickname.
// RFC 1459: https://tools.ietf.org/html/rfc1459#section-4.5.2
func (irc *Connection) Whois(nick string) {
	irc.SendRawf("WHOIS %s", nick)
}


// Query information about a given nickname in the server.
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.5.1
func (irc *Connection) Who(nick string) {
	irc.SendRawf("WHO %s", nick)
}


// Set different modes for a target (channel or nickname).
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.2.3
func (irc *Connection) Mode(target string, modestring ...string) {
	if len(modestring) > 0 {
		mode := strings.Join(modestring, " ")
		irc.SendRawf("MODE %s %s", target, mode)
		return
	}
	irc.SendRawf("MODE %s", target)
}


// A disconnect sends all buffered messages (if possible), 
// stops all goroutines and then closes the socket.
func (irc *Connection) Disconnect() {
	irc.endping <- true
	irc.endwrite <- true
	irc.endread <- true
	close(irc.pwrite)
	close(irc.pread)

	<-irc.readerExit
	<-irc.writerExit
	<-irc.pingerExit
	irc.socket.Close()
	irc.socket = nil
	if irc.netsock != nil {
		irc.netsock.Close()
		irc.netsock = nil
	}
	irc.Error <- errors.New("Disconnect Called")
}


// Reconnect to a server using the current connection.
func (irc *Connection) Reconnect() error {
	return irc.Connect(irc.server)
}


// Connect to a given server using the current connection configuration.
// This function also takes care of identification if a password is provided.
// RFC 1459 details: https://tools.ietf.org/html/rfc1459#section-4.1
func (irc *Connection) Connect(server string) error {
	irc.server = server
	irc.stopped = false

	var err error
	if irc.UseTLS {
		if irc.netsock, err = net.DialTimeout("tcp", irc.server, irc.Timeout); err == nil {
			irc.socket = tls.Client(irc.netsock, irc.TLSConfig)
		}
	} else {
		irc.socket, err = net.DialTimeout("tcp", irc.server, irc.Timeout)
	}
	if err != nil {
		return err
	}
	irc.Log.Printf("Connected to %s (%s)\n", irc.server, irc.socket.RemoteAddr())

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


// Create a connection with the (publicly visible) nickname and username.
// The nickname is later used to address the user. Returns nil if nick 
// or user are empty.
func IRC(nick, user string) *Connection {
	// catch invalid values
	if 0 == len(nick) {
		return nil
	}
	if 0 == len(user) {
		return nil
	}

	irc := &Connection{
		nick:       nick,
		user:       user,
		Log:        log.New(os.Stdout, "", log.LstdFlags),
		readerExit: make(chan bool),
		writerExit: make(chan bool),
		pingerExit: make(chan bool),
		endping:    make(chan bool),
		endread:    make(chan bool),
		endwrite:   make(chan bool),
		Version:    VERSION,
		KeepAlive:  4 * time.Minute,
		Timeout:    1 * time.Minute,
		PingFreq:   15 * time.Minute,
	}
	irc.setupCallbacks()
	return irc
}

// Returns true if all values in struct make 
// somewhat sense.
func (irc *Connection) hasValidValues() bool {
	if 0 == len(irc.nick) {
		return false
	}

	if 0 == len(irc.Version) {
		return false
	}

	if 0 == len(irc.user) {
		return false
	}

	if nil == irc.Log {
		return false
	}

	return true
}
