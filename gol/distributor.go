package gol

import (
	"strconv"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool
}

// returns the initial world as a 2-dimesional slice
func getInitialWorld(p Params, c distributorChannels) [][]uint8 {

	filePath := ("images/" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + ".pgm")

	aliveCells := util.ReadAliveCells(filePath, p.ImageWidth, p.ImageHeight)

	initialWorld := make([][]uint8, p.ImageWidth)
	for i := range initialWorld {
		initialWorld[i] = make([]uint8, p.ImageHeight)
	}

	for _, x := range aliveCells {
		initialWorld[x.X][x.Y] = 255
		cellChanged := CellFlipped{
			CompletedTurns: 0,
			Cell:           x,
		}
		c.events <- cellChanged
	}

	return initialWorld
}

func getCurrentAliveCells(world [][]uint8) []util.Cell {
	var cells []util.Cell

	for i, x := range world {
		for j, y := range x {
			if y != 0 {
				cells = append(cells, util.Cell{
					X: i,
					Y: j,
				})
			}
		}
	}

	return cells
}

const alive = 255
const dead = 0

func mod(x, m int) int {
	return (x + m) % m
}

func calculateNeighbours(x, y int, world [][]uint8) int {
	height := len(world)
	width := len(world[0])

	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(x+i, height)][mod(y+j, width)] == alive {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func calculateNextWorld(c distributorChannels, world [][]uint8, turn int) [][]uint8 {
	height := len(world)
	width := len(world[0])

	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}
	for x := 0; x < height; x++ {
		for y := 0; y < width; y++ {
			neighbours := calculateNeighbours(x, y, world)
			if world[x][y] == alive {
				if neighbours == 2 || neighbours == 3 {
					newWorld[x][y] = alive
				} else {
					c.events <- CellFlipped{
						CompletedTurns: turn,
						Cell:           util.Cell{X: x, Y: y},
					}
					newWorld[x][y] = dead
				}
			} else {
				if neighbours == 3 {
					newWorld[x][y] = alive
				} else {
					newWorld[x][y] = dead
				}
			}
		}
	}
	return newWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	// TODO: For all initially alive cells send a CellFlipped Event.

	world := getInitialWorld(p, c)

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	turn := 0

	for turn < p.Turns {
		world = calculateNextWorld(c, world, turn)
		turn++
		c.events <- TurnComplete{
			CompletedTurns: turn,
		}
	}

	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          getCurrentAliveCells(world),
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
