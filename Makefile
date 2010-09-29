<<<<<<< HEAD:Makefile
include $(GOROOT)/src/Make.inc
=======
include $(GOROOT)/src/Make.${GOARCH}
>>>>>>> 6f3c572eae2c00aaaf57248b767adf74b571ed01:Makefile

TARG=irc
GOFILES=irc.go irc_struct.go irc_callback.go

include $(GOROOT)/src/Make.pkg 
