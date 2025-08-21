package main

import (
	"github.com/pkg/profile"
)

func startProfiler(mode string) {
	switch mode {
	case "cpu":
		prof := profile.Start(profile.CPUProfile)
		defer prof.Stop()
	case "memory":
		prof := profile.Start(profile.MemProfile)
		defer prof.Stop()
	case "allocation":
		prof := profile.Start(profile.MemProfileAllocs)
		defer prof.Stop()
	}
}
