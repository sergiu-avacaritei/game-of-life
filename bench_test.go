package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

var p = gol.Params{
	Turns:       100,
	ImageWidth:  512,
	ImageHeight: 512,
}

func Benchmark512x512(b *testing.B) {
	p.Threads = 1
	for threads := 1; threads <= 16; threads = threads + 1 {
		p.Threads = threads
		b.Run(fmt.Sprintf("512x512-%v", threads), func(b *testing.B) {
			os.Stdout = nil // Disable all program output apart from benchmark results
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				b.StartTimer()
				gol.Run(p, events, nil)
				for event := range events {
					switch event.(type) {
					case gol.FinalTurnComplete:
					}
				}
				b.StopTimer()
			}
		})
	}
}
