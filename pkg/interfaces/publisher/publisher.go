package publisher

import (
	"encoders.orly/event"
	"interfaces.orly/typer"
)

type I interface {
	typer.T
	Deliver(ev *event.E)
	Receive(msg typer.T)
}

type Publishers []I
