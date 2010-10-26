
include $(GOROOT)/src/Make.inc

TARG=mango
GOFILES=\
	extract.go\
	regex.go\
	format.go\
	man.go\
	man1.go\
	man3.go\
	mango.go\

include $(GOROOT)/src/Make.cmd
