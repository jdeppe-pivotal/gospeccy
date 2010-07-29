/* 

Copyright (c) 2010 Andrea Fazzi

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

*/

package main

import (
	"spectrum"
	"sdl"
	"fmt"
	"flag"
	"os"
	"time"
)

// A Go routine for processing SDL events.
//
// Note: The first letter is uppercase, so this function is public, but it should not be.
//       The Go language fails here.
func SDL_eventLoop(evtLoop *spectrum.EventLoop, speccy *spectrum.Spectrum48k, verboseKeyboard bool) {
	ticker := time.NewTicker(/*10ms*/10*1e6)

	// Better create the event-object here once, rather than multiple times within the loop
	event := &sdl.Event{}

	for {
		select {
			case <-evtLoop.Pause:
				ticker.Stop()
				spectrum.Drain(ticker)
				evtLoop.Pause <- 0

			case <-evtLoop.Terminate:
				// Terminate this Go routine
				if evtLoop.App.Verbose { println("SDL event loop: exit") }
				ticker.Stop()
				evtLoop.Terminate <- 0
				return

			case <-ticker.C:
				if event.Poll() {
					switch event.Type {
						case sdl.QUIT:
							if evtLoop.App.Verbose { println("SDL quit -> request[exit the application]") }
							evtLoop.App.RequestExit()

						case sdl.KEYDOWN, sdl.KEYUP:
							k := event.Keyboard()

							keyName := sdl.GetKeyName(sdl.Key(k.Keysym.Sym))

							if verboseKeyboard {
								println()
								println(k.Keysym.Sym, ": ", keyName)

								fmt.Printf("%04x ", event.Type)

								for i := 0; i < len(event.Pad0); i++ {
									fmt.Printf("%02x ", event.Pad0[i])
								}
								println()

								fmt.Printf("Type: %02x Which: %02x State: %02x Pad: %02x\n", k.Type, k.Which, k.State, k.Pad0[0])
								fmt.Printf("Scancode: %02x Sym: %08x Mod: %04x Unicode: %04x\n", k.Keysym.Scancode, k.Keysym.Sym, k.Keysym.Mod, k.Keysym.Unicode)
							}

							if keyName == "escape" {
								if evtLoop.App.Verbose { println("escape key -> request[exit the application]") }
								evtLoop.App.RequestExit()
							} else {
								sequence, haveMapping := spectrum.SDL_KeyMap[keyName]

								if haveMapping {
									switch event.Type {
									case sdl.KEYDOWN:
										// Normal order
										for i:=0; i<len(sequence); i++ {
											speccy.Keyboard.KeyDown(sequence[i])
										}
									case sdl.KEYUP:
										// Reverse order
										for i:=len(sequence)-1; i>=0; i-- {
											speccy.Keyboard.KeyUp(sequence[i])
										}
									}
								}
							}
					}
				}
		}
	}
}

func emulatorLoop(evtLoop *spectrum.EventLoop, speccy *spectrum.Spectrum48k, displayRefreshFrequency float) {
	ticker := time.NewTicker(int64(1e9/displayRefreshFrequency))

	for {
		select {
			case <-evtLoop.Pause:
				ticker.Stop()
				speccy.Close(evtLoop.App.Verbose)
				spectrum.Drain(ticker)
				evtLoop.Pause <- 0

			case <-evtLoop.Terminate:
				// Terminate this Go routine
				if evtLoop.App.Verbose { println("emulator loop: exit") }
				evtLoop.Terminate <- 0
				return

			case <-ticker.C:
				// if evtLoop.App.Verbose { fmt.Printf("%d ms\n", time.Nanoseconds()/1e6) }
				speccy.RenderFrame()
		}
	}
}

func main() {
	help := flag.Bool("help", false, "Show usage")
	scale2x := flag.Bool("2x", false, "2x display scaler")
	fullscreen:= flag.Bool("fullscreen", false, "Fullscreen (enable 2x scaler by default)")
	fps := flag.Float("fps", 50.08, "Frames per second")
	verbose := flag.Bool("verbose", false, "Enable debugging messages")
	verboseKeyboard := flag.Bool("verbose-keyboard", false, "Enable debugging messages (keyboard events)")

	{
		flag.Usage = func() {
			fmt.Fprintf(os.Stderr, "GoSpeccy - A simple ZX Spectrum 48k Emulator written in GO\n\n")
			fmt.Fprintf(os.Stderr, "Usage:\n\n")
			fmt.Fprintf(os.Stderr, "\tgospeccy [options] [image.sna]\n\n")
			fmt.Fprintf(os.Stderr, "Options are:\n\n")
			flag.PrintDefaults()
		}

		flag.Parse()

		if *help == true {
			flag.Usage()
			return
		}
	}

	app := spectrum.NewApplication()
	app.Verbose = *verbose

	// Create new emulator core
	speccy, err := spectrum.NewSpectrum48k()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		app.RequestExit()
		goto quit
	}

	// Load snapshot (if any)
	if flag.Arg(0) != "" {
		if app.Verbose { fmt.Printf("loading snapshot \"%s\"\n", flag.Arg(0)) }
		err := speccy.LoadSna(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			app.RequestExit()
			goto quit
		}
	}

	// Setup the display
	{
		if sdl.Init(sdl.INIT_EVERYTHING) != 0 {
			panic(sdl.GetError())
		}

		if *fullscreen {
			*scale2x = true
		}

		if *scale2x {
			speccy.AddDisplay(spectrum.NewSDLScreen2x(app, *fullscreen))
		} else {
			speccy.AddDisplay(spectrum.NewSDLScreen(app))
		}

		sdl.WM_SetCaption("GoSpeccy - ZX Spectrum Emulator", "")
	}

	// Begin speccy emulation
	go SDL_eventLoop(app.NewEventLoop(), speccy, *verboseKeyboard)
	go emulatorLoop(app.NewEventLoop(), speccy, *fps)

quit:
	<-app.HasTerminated
	sdl.Quit()
}
