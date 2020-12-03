package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

var p = gol.Params{
	Turns:       100,
	ImageWidth:  64,
	ImageHeight: 64,
}

func Benchmark(b *testing.B) {
	p.Threads = 1
	for threads := 1; threads <= 16; threads++ {
		p.Threads = threads
		b.Run(fmt.Sprintf("64x64-%v", threads), func(b *testing.B) {
			os.Stdout = nil // Disable all program output apart from benchmark results
			events := make(chan gol.Event)
			for i := 0; i < b.N; i++ {
				b.StartTimer()
				gol.Run(p, events, nil)
				b.StopTimer()
			}
		})
	}
}
