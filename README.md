Grace
==============

Grace  is a very simple debugging/tracing framework written in the Go
programming language. It is currently very young and doesn't do much,
but does implment loading processes, setting arbitrary callbacks for
breakpoints, and more importantly letting you set breakpoints
symbolically in the presence of DWARF symbols.

Grace is also an awful example of idiomatic Go code, but that will
change eventually.


To build, simply extract it anywhere and run:

    GOPATH=`pwd` go build grace.go
    ./grace

(So far, Linux only)

Check `grace.go` for basic usage info.

Questions?  @yan / yan@srtd.org
