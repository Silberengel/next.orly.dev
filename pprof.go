package main

import (
	"github.com/pkg/profile"
	"utils.orly/interrupt"
)

func startProfiler(mode string) {
	switch mode {
	case "cpu":
		prof := profile.Start(profile.CPUProfile)
		interrupt.AddHandler(prof.Stop)
	case "memory":
		prof := profile.Start(profile.MemProfile)
		interrupt.AddHandler(prof.Stop)
	case "allocation":
		prof := profile.Start(profile.MemProfileAllocs)
		interrupt.AddHandler(prof.Stop)
	}
}
