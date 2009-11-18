all:
	8g irc.go irc_struct.go
	8g test.go
	8l -o test test.8

clean:
	rm *.8 *.6 test
