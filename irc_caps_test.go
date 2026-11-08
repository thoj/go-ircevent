package irc

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestConnectRequestedCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name        string
		requested   []string
		sasl        bool
		nak         bool
		missingSASL bool
		want        []string
	}{
		{name: "none"},
		{name: "custom", requested: []string{"message-tags"}, want: []string{"message-tags"}},
		{name: "unavailable", requested: []string{"not-supported"}},
		{name: "rejected", requested: []string{"message-tags"}, nak: true, want: []string{"message-tags"}},
		{name: "unsupported_sasl", requested: []string{"message-tags"}, sasl: true, missingSASL: true},
		{name: "sasl", sasl: true, want: []string{"sasl"}},
		{name: "sasl_and_custom", requested: []string{"message-tags"}, sasl: true, want: []string{"message-tags", "sasl"}},
		{name: "explicit_sasl_and_custom", requested: []string{"message-tags", "sasl"}, sasl: true, want: []string{"message-tags", "sasl"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()

			client := IRC("nick", "user")
			client.Log = log.New(new(bytes.Buffer), "", 0)
			client.Timeout = time.Second
			requestedBacking := append(append([]string(nil), tc.requested...), "sentinel")
			client.RequestCaps = requestedBacking[:len(tc.requested)]
			client.UseSASL = tc.sasl
			client.SASLLogin, client.SASLPassword = "user", "password"

			// Reuse the public connection configuration across two connections.
			for attempt := 0; attempt < 2; attempt++ {
				type result struct {
					requested []string
					err       error
				}
				serverResult := make(chan result, 1)
				go func() {
					conn, err := listener.Accept()
					if err != nil {
						serverResult <- result{err: err}
						return
					}
					defer conn.Close()
					conn.SetDeadline(time.Now().Add(3 * time.Second))
					var requested []string
					authenticating := false
					negotiating := false
					scanner := bufio.NewScanner(conn)
					for scanner.Scan() {
						line := scanner.Text()
						switch {
						case line == "CAP LS":
							negotiating = true
							offered := "message-tags sasl"
							if tc.missingSASL {
								offered = "message-tags"
							}
							fmt.Fprintf(conn, ":server CAP nick LS :%s\r\n", offered)
						case strings.HasPrefix(line, "CAP REQ :"):
							capability := strings.TrimPrefix(line, "CAP REQ :")
							requested = append(requested, capability)
							command := "ACK"
							if tc.nak {
								command = "NAK"
							}
							fmt.Fprintf(conn, ":server CAP nick %s :%s\r\n", command, capability)
						case line == "AUTHENTICATE PLAIN":
							authenticating = true
							fmt.Fprint(conn, "AUTHENTICATE +\r\n")
						case strings.HasPrefix(line, "AUTHENTICATE ") && authenticating:
							fmt.Fprint(conn, ":server 903 nick :SASL successful\r\n")
						case line == "CAP END":
							negotiating = false
						case strings.HasPrefix(line, "USER "):
							if negotiating {
								serverResult <- result{err: fmt.Errorf("registration arrived before CAP END")}
								return
							}
							serverResult <- result{requested: requested}
							return
						}
					}
					serverResult <- result{requested: requested, err: scanner.Err()}
				}()

				if attempt == 0 {
					err = client.Connect(listener.Addr().String())
				} else {
					err = client.Reconnect()
				}
				if tc.missingSASL {
					close(client.end)
					client.socket.Close()
					client.Wait()
					<-serverResult
					if err == nil || !strings.Contains(err.Error(), "no SASL capability") {
						t.Errorf("Connect() = %v, want unsupported SASL error", err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				got := <-serverResult
				// The server closes its socket once registration is received, so
				// stop and join all connection goroutines before reconnecting.
				close(client.end)
				client.socket.Close()
				client.Wait()
				if got.err != nil {
					t.Fatal(got.err)
				}
				sort.Strings(got.requested)
				if !reflect.DeepEqual(got.requested, tc.want) {
					t.Errorf("attempt %d requests = %v, want %v", attempt, got.requested, tc.want)
				}
				if len(client.RequestCaps) != len(tc.requested) || strings.Join(client.RequestCaps, " ") != strings.Join(tc.requested, " ") {
					t.Errorf("attempt %d changed RequestCaps to %v, want %v", attempt, client.RequestCaps, tc.requested)
				}
				if requestedBacking[len(tc.requested)] != "sentinel" {
					t.Error("negotiation overwrote the caller's capability backing array")
				}
				acknowledged := append([]string(nil), client.AcknowledgedCaps...)
				sort.Strings(acknowledged)
				wantAcknowledged := tc.want
				if tc.nak {
					wantAcknowledged = nil
				}
				if !reflect.DeepEqual(acknowledged, wantAcknowledged) {
					t.Errorf("attempt %d acknowledgments = %v, want %v", attempt, acknowledged, wantAcknowledged)
				}
			}
		})
	}
}
