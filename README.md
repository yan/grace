Grace
==============

Grace (gotrace-ot) is a very simple debugging/tracing framework written in the
Go programming language. It is currently very young and doesn't do much, but
does implment loading processes and setting arbitrary callbacks for breakpoints.

Grace is also an awful example of idiomatic Go code, but that will change
eventually.

To build, simply extract it anywhere and run:

    GOPATH=`pwd` go build grace.go
    ./grace

Check `grace.go` for basic usage info.

Questions?  @yan / yan@srtd.org
