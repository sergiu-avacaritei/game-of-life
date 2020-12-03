package gol

import (
	"fmt"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

const alive = 255
const dead = 0

type distributorChannels struct {
	events        chan<- Event
	ioCommand     chan<- ioCommand
	ioIdle        <-chan bool
	ioFilename    chan<- string
	ioOutput      chan<- uint8
	ioInput       <-chan uint8
	sdlKeyPresses <-chan rune
	stopResume    [16]chan bool
}

func mod(x, m int) int {
	return (x + m) % m
}

// Return the initial world as a 2D slice.
func getInitialWorld(p Params, c distributorChannels) [][]uint8 {

	initialWorld := make([][]byte, p.ImageHeight)
	for i := range initialWorld {
		initialWorld[i] = make([]byte, p.ImageWidth)
	}

	for x := 0; x < p.ImageHeight; x++ {
		for y := 0; y < p.ImageWidth; y++ {
			initialWorld[y][x] = <-c.ioInput // (Y,X) !!!!!!
		}
	}

	aliveCells := getCurrentAliveCells(initialWorld) // Could do this in the for-loop above! (tiny improvement)

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

// Return all alive cells.
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

func doThis() {
	fmt.Println("Hi Bitch")
}

func calculateNextWorld(c distributorChannels, chunk chan [][]uint8, turn int, offset int, routineNumber int) {
	world := <-chunk

	height := len(world)
	width := len(world[0])

	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}

	for x := 1; x < height-1; x++ {
		for y := 0; y < width; y++ {

			select {
			case coco := <-c.stopResume[routineNumber]:
				fmt.Println("I Stopped ", coco)
				<-c.stopResume[routineNumber]
				fmt.Println("I resumed ")
			default:
			}

			neighbours := calculateNeighbours(x, y, world)
			if world[x][y] == alive {
				if neighbours == 2 || neighbours == 3 {
					newWorld[x][y] = alive
				} else {
					c.events <- CellFlipped{
						CompletedTurns: turn,
						Cell:           util.Cell{X: x + offset - 1, Y: y},
					}
					newWorld[x][y] = dead
				}
			} else {
				if neighbours == 3 {
					c.events <- CellFlipped{
						CompletedTurns: turn,
						Cell:           util.Cell{X: x + offset - 1, Y: y},
					}
					newWorld[x][y] = alive
				} else {
					newWorld[x][y] = dead
				}
			}
		}
	}
	newWorld = newWorld[1:(height - 1)]
	chunk <- newWorld
}

func calculateParallelStep(p Params, c distributorChannels, turn int, world [][]uint8) [][]uint8 {

	chunk := make([]chan [][]byte, p.Threads)
	worldsChunk := make([][][]uint8, p.Threads)

	chunkWidth := p.ImageWidth / p.Threads

	if p.Threads == 1 {
		worldsChunk[0] = append([][]byte{world[p.ImageWidth-1]}, world...)
		worldsChunk[0] = append(worldsChunk[0], [][]byte{world[0]}...)
	} else {
		//Making the world for the first thread
		worldsChunk[0] = append([][]byte{world[p.ImageWidth-1]}, world[0:chunkWidth+1]...)
		// Making the world for the last thread (if there is more than one thread)
		worldsChunk[p.Threads-1] = append(world[((p.Threads-1)*chunkWidth-1):], [][]byte{world[0]}...)
	}

	var newWorld [][]byte
	for i := 0; i < p.Threads; i++ { // Making the worlds for all the threads exept the first and the last
		// ((chunkWidth * i) - 1) -> (chunkWidth * (i+1))
		if i != 0 && i != p.Threads-1 {
			worldsChunk[i] = world[(chunkWidth*i - 1):(chunkWidth*(i+1) + 1)]
		}
		offset := i * chunkWidth
		chunk[i] = make(chan [][]byte)
		go calculateNextWorld(c, chunk[i], turn, offset, i)
		chunk[i] <- worldsChunk[i]
	}

	for i := 0; i < p.Threads; i++ {
		newWorld = append(newWorld, <-chunk[i]...)
	}

	return newWorld
}

func writePgm(p Params, c distributorChannels, turn int, world [][]uint8) {
	fileName := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
	c.ioCommand <- 0
	c.ioFilename <- fileName
	for x := 0; x < p.ImageHeight; x++ {
		for y := 0; y < p.ImageWidth; y++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.events <- ImageOutputComplete{turn, fileName}
}

func pauseNow(done chan bool, p Params, c distributorChannels, ticker *ticker, turn *int) {
	for i := 0; i < p.Threads; i++ {
		c.stopResume[i] <- true
	}
	ticker.stopTicker(done)
	c.events <- StateChange{*turn, Paused}
}

func resumeNow(done chan bool, p Params, c distributorChannels, ticker *ticker, turn *int, world *[][]byte) {
	for i := 0; i < p.Threads; i++ {
		c.stopResume[i] <- false
	}
	ticker.resetTicker(c, turn, world, done)
	fmt.Println("Continuing")
	c.events <- StateChange{*turn, Executing}
}

func manageSdlInput(p Params, c distributorChannels, turn *int, world *[][]uint8, done chan bool, ticker *ticker) bool {
	select {
	case x := <-c.sdlKeyPresses:
		if x == 's' {
			writePgm(p, c, *turn, *world)
		} else if x == 'q' {
			writePgm(p, c, *turn, *world)
			closeProgramm(c, *turn, done, ticker)
			return true
		} else if x == 'p' {
			pauseNow(done, p, c, ticker, turn)
			resume := 'N'
			for resume != 'p' {
				resume = <-c.sdlKeyPresses
				if resume == 's' {
					writePgm(p, c, *turn, *world)
				} else if resume == 'q' {
					writePgm(p, c, *turn, *world)
					c.ioCommand <- ioCheckIdle
					<-c.ioIdle
					c.events <- StateChange{*turn, Quitting}
					close(c.events)
					return true
				}
			}
			resumeNow(done, p, c, ticker, turn, world)

		}
	default:
		return false
	}
	return false
}

func closeProgramm(c distributorChannels, turn int, done chan bool, ticker *ticker) {
	ticker.ticker.Stop()
	done <- true

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

type ticker struct {
	period time.Duration
	ticker time.Ticker
}

func createTicker(period time.Duration) *ticker {
	return &ticker{period, *time.NewTicker(period)}
}

func (t *ticker) stopTicker(done chan bool) {
	t.ticker.Stop()
	done <- true
}

func (t *ticker) resetTicker(c distributorChannels, turn *int, world *[][]uint8, done chan bool) {
	t.ticker = *time.NewTicker(t.period)
	tickerRun(c, turn, world, done, t)
}

func tickerRun(c distributorChannels, turn *int, world *[][]uint8, done chan bool, ticker *ticker) {

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.ticker.C:
				c.events <- AliveCellsCount{*turn, len(getCurrentAliveCells(*world))}
			}
		}
	}()
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// READ
	c.ioCommand <- 1
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	// TODO: Create a 2D slice to store the world.
	// TODO: For all initially alive cells send a CellFlipped Event.
	world := getInitialWorld(p, c)

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	turn := 0

	ticker := createTicker(2 * time.Second)
	done := make(chan bool)

	tickerRun(c, &turn, &world, done, ticker)

	for turn < p.Turns {

		if manageSdlInput(p, c, &turn, &world, done, ticker) {
			return
		}

		world = calculateParallelStep(p, c, turn, world)

		turn++
		c.events <- TurnComplete{
			CompletedTurns: turn,
		}
	}

	// WRITE
	writePgm(p, c, turn, world)

	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          getCurrentAliveCells(world),
	}

	closeProgramm(c, turn, done, ticker)
}
