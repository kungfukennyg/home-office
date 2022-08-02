package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosuri/uilive"
	"github.com/kungfukennyg/home-office/cync-lights/log"
	"github.com/pkg/errors"
	"github.com/unixpickle/cbyge"
)

type Mode interface {
	onSwitch(*controller) error
	run(*controller) (time.Duration, error)
	onExit(*controller)
	getId() string
	isIndefinite() bool
}

const ModeCommandID = "command"

type ModeCommand struct {
}

func (mc *ModeCommand) onSwitch(cont *controller) error {
	return nil
}

func (mc ModeCommand) run(cont *controller) (time.Duration, error) {
	// get a command
	outputWriter := log.New()
	outputWriter.Start()
	defer outputWriter.Stop()
	log.FPrintln(outputWriter, "Enter command (h for help):")

	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	command := strings.Trim(input.Text(), "\n")
	args := strings.Split(command, " ")
	if len(args) < 1 {
		return time.Second, nil
	}

	switch strings.ToLower(args[0]) {
	case "h", "help":
		log.FPrintf(outputWriter, "help, on, off, modes, switchmode, printdevices, exit\n")
	case "switchmode":
		if len(args) < 2 || strings.Trim(args[1], " ") == "" {
			log.FPrintf(outputWriter, "Usage: switchMode <mode>\n")
			return time.Second, nil
		}
		err := cont.SwitchMode(args[1])
		if err != nil {
			return time.Second, errors.Wrap(err, "failed to switch modes")
		}
	case "printdevices":
		cont.refreshDeviceCache()
		// TODO: proper logging
		cont.PrintDevices()
	case "turnoff", "off":
		for _, d := range cont.devices {
			cont.SetStatus(d, false)
		}
	case "turnon", "on":
		for _, d := range cont.devices {
			cont.SetStatus(d, true)
		}
	case "listmodes", "modes":
		line := "\t"
		for id := range cont.modes {
			line += id + ", "
		}
		line = line[:len(line)-2] + "\n"
		log.FPrintf(outputWriter, line)
	case "exit":
		cont.running = false
	default:
		log.FPrintf(outputWriter, "unrecognized command %s", command)
	}
	return 100 * time.Millisecond, nil
}

func (mc ModeCommand) onExit(cont *controller) {
	//
}

func (mc ModeCommand) isIndefinite() bool {
	return false
}

func (mc ModeCommand) getId() string {
	return ModeCommandID
}

const ModeRainbowID = "rainbow"

type ModeRainbow struct {
	hue  float64
	rgbs map[string]*RGB
	pos  RGB
}

type RGB [3]uint8

func NewRGB(rgb [3]uint8) RGB {
	return RGB{rgb[0], rgb[1], rgb[2]}
}

func (rgb *RGB) equals(other [3]uint8) bool {
	return rgb[0] == other[0] && rgb[1] == other[1] && rgb[2] == other[2]
}

func (rgb *RGB) sub(other RGB) RGB {
	return RGB{rgb[0] - other[0], rgb[1] - other[1], rgb[2] - other[2]}
}

func (rgb *RGB) incrementBy(ret uint8) {
	rgb[0] += ret
	rgb[1] += ret
	rgb[2] += ret
}

func (rgb *RGB) set(r uint8, g uint8, b uint8) {
	rgb[0] = r
	rgb[1] = g
	rgb[2] = b
}

func (mc *ModeRainbow) onSwitch(cont *controller) error {
	return nil
}

func (mc *ModeRainbow) run(cont *controller) (time.Duration, error) {
	// increment each color once in 3 total iterations
	for r := mc.pos[0]; r < r+1; r++ {
		for g := mc.pos[1]; g < g+1; g++ {
			for b := mc.pos[2]; b < b+1; b++ {
				for _, device := range cont.devices {
					if !device.LastStatus().IsOnline {
						// not responding, back off
						fmt.Printf("device %+v not responding, backing off...\n", device)
						continue
					}

					fmt.Printf("incrementing rgb for device %s from [%+v]", device.Name(), device.LastStatus().StatusPaginatedResponse.RGB)
					fmt.Printf(" to [%d, %d, %d]\n", r, g, b)
					err := cont.SetRGB(device, RGB{uint8(r), uint8(g), uint8(b)})
					if err != nil {
						fmt.Printf("[ModeRainbow.run] failed to set device RGB: %v, continuing...\n", err)
						continue
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	mc.pos[0]++
	mc.pos[1]++
	mc.pos[2]++

	return 250 * time.Millisecond, nil
}

func pointer[Value any](val Value) *Value {
	return &val
}

func (mc ModeRainbow) onExit(cont *controller) {
	//
}

func (mc ModeRainbow) isIndefinite() bool {
	return true
}

func (mc ModeRainbow) getId() string {
	return ModeRainbowID
}

var prettyColors = []RGB{
	{255, 0, 0},   // red
	{255, 128, 0}, // orange
	{255, 255, 0}, // yellow
	// {128, 255, 0}, // yellow-green
	{0, 255, 0}, // green
	// {0, 255, 128}, // teal-green
	{0, 255, 255}, // teal
	// {0, 128, 255}, // light-blue
	{0, 0, 255},   // blue
	{127, 0, 255}, // purple
	{255, 0, 255}, // pink
	{255, 0, 127}, // red-pink
}

var colorNames = map[RGB]string{
	prettyColors[0]: "red",
	prettyColors[1]: "orange",
	prettyColors[2]: "yellow",
	// prettyColors[3]: "yellow-green",
	prettyColors[3]: "green",
	// prettyColors[0]: "teal-green",
	prettyColors[4]: "teal",
	// prettyColors[4]: "light-blue",
	prettyColors[5]: "blue",
	prettyColors[6]: "purple",
	prettyColors[7]: "pink",
	prettyColors[8]: "red-pink",
}

const ModePrettyID = "pretty"

type ModePretty struct {
	colorIndices   map[string]int
	randomColors   bool
	slowTransition bool
	lastColor      map[string]RGB

	// logging writers
	writer     *uilive.Writer
	otherLines map[string]io.Writer
}

func (mc *ModePretty) onSwitch(cont *controller) error {
	mc.writer = log.New()

	mc.writer.Start()

	input := scanInputV2(mc.writer, "Choose colors randomly? (true/t/false/f)")
	randomColors, err := strconv.ParseBool(input)
	if err != nil {
		return errors.Wrap(err, "value must be one of: (true/t/false/f)")
	}
	mc.randomColors = randomColors

	input = scanInputV2(mc.writer, "Slow transition colors? (true/t/false/f)")
	slowTransition, err := strconv.ParseBool(input)
	if err != nil {
		return errors.Wrap(err, "value must be one of: (true/t/false/f)")
	}
	log.FPrintln(mc.writer, fmt.Sprintf("Starting %s mode...", ModePrettyID))
	mc.slowTransition = slowTransition
	rand.Seed(time.Now().UnixNano())
	mc.writer.Stop()
	mc.writer = nil
	return nil
}

func (mc *ModePretty) run(cont *controller) (time.Duration, error) {
	// failed experiment to back off when devices get unresponsive -- in practice polling statuses
	// introduces more unresponsiveness than just yeeting the updates off
	// _, errors := cont.refreshDeviceCache()
	// if len(errors) > 0 {
	// 	// some devices unresponsive
	// 	var realError error
	// 	for _, err = range errors {
	// 		if err != nil {
	// 			realError = err
	// 		}
	// 	}
	// 	if realError != nil {
	// 		fmt.Println("[ModePretty.run] devices unresponsive, backing off...")
	// 		time.Sleep(1 * time.Second)
	// 		fmt.Printf("got error: %v\n", err)
	// 	}
	// }

	if mc.writer == nil {
		mc.writer = log.New()
		devices := make(map[string]io.Writer, len(cont.devices))
		for _, d := range cont.devices {
			if cont.debug {
				fmt.Printf("creating writer for device %s\n", d.Name())
			}
			devices[d.DeviceID()] = mc.writer.Newline()
		}
		mc.otherLines = devices
		mc.writer.Start()
	}

	var wg sync.WaitGroup
	wg.Add(len(cont.devices))
	log.FPrintf(mc.writer, "Pretty mode (random: %v, smoothTransition: %v)...\n", mc.randomColors, mc.slowTransition)
	for _, device := range cont.devices {
		var color RGB
		if mc.randomColors {
			lastColor, ok := mc.lastColor[device.DeviceID()]
			for {
				color = prettyColors[indexRandom(cont)]
				if !ok || (ok && !lastColor.equals(color)) {
					break
				}
			}
		} else {
			color = prettyColors[mc.indexIncremental(device)]
		}

		if mc.slowTransition {
			// last, ok := mc.lastColor[device.DeviceID()]
			// if !ok {
			// 	last = prettyColors[indexRandom(cont)]
			// }
			cont.smoothTransitionLum(device, color, 2, &wg, 50*time.Millisecond)
			// cont.smoothTransitionRGB(device, last, color, 4, &wg, 2000*time.Millisecond)
		} else {
			cont.SetRGBAsync(device, color)
		}
		mc.lastColor[device.DeviceID()] = color
		deviceWriter := mc.otherLines[device.DeviceID()]
		rgbStr := "["
		for _, val := range color {
			rgbStr += fmt.Sprintf("%03d, ", val)
		}
		rgbStr = rgbStr[:len(rgbStr)-2] + "]"
		log.FPrintf(deviceWriter, "%s (%s) - %s\n", rgbStr, colorNames[color], device.Name())
	}

	if mc.slowTransition {
		wg.Wait()
	}

	log.FPrintln(mc.writer, "")

	return 50 * time.Millisecond, nil
}

func indexRandom(cont *controller) int {
	return rand.Intn(len(prettyColors))
}

func (mc *ModePretty) indexIncremental(device *cbyge.ControllerDevice) int {
	i, ok := mc.colorIndices[device.DeviceID()]
	if !ok {
		i = 0
		mc.colorIndices[device.DeviceID()] = i
	}

	i = i + 1
	if i >= len(prettyColors) {
		i = 0
	}
	mc.colorIndices[device.DeviceID()] = i
	return i
}

func (mc ModePretty) onExit(cont *controller) {
	log.FPrintln(mc.writer, "Exiting pretty mode...")
	mc.writer.Stop()
	//
}

func (mc ModePretty) isIndefinite() bool {
	return true
}

func (mc ModePretty) getId() string {
	return ModePrettyID
}

const ModeExperimentID = "experiment"

type ModeExperiment struct {
}

func (mc *ModeExperiment) onSwitch(cont *controller) error {
	return nil
}

func (mc *ModeExperiment) run(cont *controller) (time.Duration, error) {
	for _, device := range cont.devices {
		colors := scanInput("ModeExperiment.run", fmt.Sprintf("Enter color for device %s", device.Name()))
		split := strings.Split(colors, " ")
		if len(split) < 3 {
			if len(split) > 0 && split[0] == "exit" {
				cont.SwitchMode(ModeCommandID)
				return 50 * time.Millisecond, nil
			}

			fmt.Printf("[ModeExperiment.run] must specify colors as space-separated RGB 256 numbers...")
			return 50 * time.Millisecond, nil
		}

		r, _ := strconv.ParseUint(split[0], 10, 8)
		g, _ := strconv.ParseUint(split[1], 10, 8)
		b, _ := strconv.ParseUint(split[2], 10, 8)
		cont.SetRGB(device, RGB{uint8(r), uint8(g), uint8(b)})
	}

	return 50 * time.Millisecond, nil
}

func (mc ModeExperiment) onExit(cont *controller) {
	//
}

func (mc ModeExperiment) isIndefinite() bool {
	return false
}

func (mc ModeExperiment) getId() string {
	return ModeExperimentID
}
