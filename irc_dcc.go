package irc

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

// DCCChat reprezentuje pojedyncze połączenie DCC CHAT
type DCCChat struct {
	Nick     string
	Conn     net.Conn
	Incoming chan string
	Outgoing chan string
	mutex    sync.Mutex
}

// DCCManager zarządza wszystkimi połączeniami DCC
type DCCManager struct {
	chats map[string]*DCCChat
	mutex sync.Mutex
}

// NewDCCManager tworzy nowy menedżer DCC
func NewDCCManager() *DCCManager {
	return &DCCManager{
		chats: make(map[string]*DCCChat),
	}
}

func (irc *Connection) handleIncomingDCCChat(nick string, ip net.IP, port int) {
	addr := fmt.Sprintf("%s:%d", ip.String(), port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		irc.Log.Printf("Error connecting to DCC CHAT from %s: %v", nick, err)
		return
	}

	chat := &DCCChat{
		Nick:     nick,
		Conn:     conn,
		Incoming: make(chan string, 100),
		Outgoing: make(chan string, 100),
	}

	irc.DCCManager.mutex.Lock()
	irc.DCCManager.chats[nick] = chat
	irc.DCCManager.mutex.Unlock()

	go irc.handleDCCChatConnection(chat)
}

func (irc *Connection) handleDCCChatConnection(chat *DCCChat) {
	defer chat.Conn.Close()
	defer func() {
		irc.DCCManager.mutex.Lock()
		delete(irc.DCCManager.chats, chat.Nick)
		irc.DCCManager.mutex.Unlock()
	}()

	readDone := make(chan struct{})
	writeDone := make(chan struct{})

	go func() {
		irc.readDCCChat(chat)
		close(readDone)
	}()

	go func() {
		irc.writeDCCChat(chat)
		close(writeDone)
	}()

	select {
	case <-readDone:
		irc.Log.Printf("DCC CHAT read routine finished for %s", chat.Nick)
	case <-writeDone:
		irc.Log.Printf("DCC CHAT write routine finished for %s", chat.Nick)
	}

	irc.Log.Printf("DCC CHAT connection closed with %s", chat.Nick)
}

func (irc *Connection) readDCCChat(chat *DCCChat) {
	scanner := bufio.NewScanner(chat.Conn)
	for scanner.Scan() {
		chat.Incoming <- scanner.Text()
	}
	close(chat.Incoming)
}

func (irc *Connection) writeDCCChat(chat *DCCChat) {
	for msg := range chat.Outgoing {
		_, err := fmt.Fprintf(chat.Conn, "%s\r\n", msg)
		if err != nil {
			irc.Log.Printf("Error writing to DCC CHAT with %s: %v", chat.Nick, err)
			break
		}
	}
	close(chat.Outgoing)
}
func (irc *Connection) InitiateDCCChat(target string) error {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("error creating listener for DCC CHAT: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	ip := irc.getLocalIP()

	irc.SendRawf("PRIVMSG %s :\001DCC CHAT chat %d %d\001", target, ip2int(ip), port)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			irc.Log.Printf("Error accepting DCC CHAT connection: %v", err)
			return
		}
		listener.Close()

		chat := &DCCChat{
			Nick:     target,
			Conn:     conn,
			Incoming: make(chan string, 100),
			Outgoing: make(chan string, 100),
		}

		irc.DCCManager.mutex.Lock()
		irc.DCCManager.chats[target] = chat
		irc.DCCManager.mutex.Unlock()

		go irc.handleDCCChatConnection(chat)
	}()

	return nil
}

func (irc *Connection) getLocalIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return net.ParseIP("127.0.0.1")
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}
func (irc *Connection) SendDCCMessage(nick, message string) error {
	irc.DCCManager.mutex.Lock()
	chat, exists := irc.DCCManager.chats[nick]
	irc.DCCManager.mutex.Unlock()

	if !exists {
		return fmt.Errorf("no active DCC chat with %s", nick)
	}

	select {
	case chat.Outgoing <- message:
		return nil
	default:
		return fmt.Errorf("failed to send message to %s: channel full", nick)
	}
}

func (irc *Connection) GetDCCMessage(nick string) (string, error) {
	irc.DCCManager.mutex.Lock()
	chat, exists := irc.DCCManager.chats[nick]
	irc.DCCManager.mutex.Unlock()

	if !exists {
		return "", fmt.Errorf("no active DCC chat with %s", nick)
	}

	select {
	case msg, ok := <-chat.Incoming:
		if !ok {
			return "", fmt.Errorf("DCC chat with %s closed", nick)
		}
		return msg, nil
	default:
		return "", fmt.Errorf("no message available from %s", nick)
	}
}

// Dodaj te metody do pliku irc_dcc.go

// CloseDCCChat zamyka połączenie DCC CHAT z określonym nickiem
func (irc *Connection) CloseDCCChat(nick string) error {
	irc.DCCManager.mutex.Lock()
	defer irc.DCCManager.mutex.Unlock()

	chat, exists := irc.DCCManager.chats[nick]
	if !exists {
		return fmt.Errorf("no active DCC chat with %s", nick)
	}

	close(chat.Outgoing)
	chat.Conn.Close()
	delete(irc.DCCManager.chats, nick)
	return nil
}

// ListActiveDCCChats zwraca listę nicków, z którymi mamy aktywne połączenia DCC CHAT
func (irc *Connection) ListActiveDCCChats() []string {
	irc.DCCManager.mutex.Lock()
	defer irc.DCCManager.mutex.Unlock()

	var activeChats []string
	for nick := range irc.DCCManager.chats {
		activeChats = append(activeChats, nick)
	}
	return activeChats
}

// IsDCCChatActive sprawdza, czy istnieje aktywne połączenie DCC CHAT z danym nickiem
func (irc *Connection) IsDCCChatActive(nick string) bool {
	irc.DCCManager.mutex.Lock()
	defer irc.DCCManager.mutex.Unlock()

	_, exists := irc.DCCManager.chats[nick]
	return exists
}

// SetDCCChatTimeout ustawia timeout dla połączeń DCC CHAT
func (irc *Connection) SetDCCChatTimeout(timeout time.Duration) {
	irc.DCCManager.mutex.Lock()
	defer irc.DCCManager.mutex.Unlock()

	for _, chat := range irc.DCCManager.chats {
		chat.Conn.SetDeadline(time.Now().Add(timeout))
	}
}
