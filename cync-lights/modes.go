package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gosuri/uilive"
	"github.com/kungfukennyg/home-office/cync-lights/log"
	"github.com/pkg/errors"
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
	log.FPrintln(outputWriter, log.MainColor, "Enter command (h for help):")

	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	command := strings.Trim(input.Text(), "\n")
	args := strings.Split(command, " ")
	if len(args) < 1 {
		return time.Second, nil
	}

	switch strings.ToLower(args[0]) {
	case "h", "help":
		var modes string
		for _, mode := range cont.modes {
			if mode.getId() == ModeCommandID {
				continue
			}

			modes += mode.getId() + ", "
		}
		modes = modes[:len(modes)-2]
		log.FPrintf(outputWriter, log.MainColor, "help, on, off, printdevices, exit, %s\n", modes)
	case "printdevices":
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
	case "exit":
		cont.running = false
	case ModeExperimentID, ModeRainbowID:
		err := cont.SwitchMode(args[0])
		if err != nil {
			return time.Millisecond, errors.Wrap(err, "failed to switch modes")
		}
	default:
		log.FPrintf(outputWriter, log.OutputColor, "unrecognized command %s\n", command)
	}

	return 1 * time.Millisecond, nil
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

var prettyColors = []RGB{
	{255, 0, 0},   // red
	{255, 128, 0}, // orange
	{255, 255, 0}, // yellow
	{128, 255, 0}, // yellow-green
	{0, 255, 0},   // green
	{0, 255, 128}, // teal-green
	{0, 255, 255}, // teal
	{0, 128, 255}, // light-blue
	{0, 0, 255},   // blue
	{127, 0, 255}, // purple
	{255, 0, 255}, // pink
	{255, 0, 127}, // red-pink
}

var colorNames = map[RGB]string{
	prettyColors[0]:  "red",
	prettyColors[1]:  "orange",
	prettyColors[2]:  "yellow",
	prettyColors[3]:  "yellow-green",
	prettyColors[4]:  "green",
	prettyColors[5]:  "teal-green",
	prettyColors[6]:  "teal",
	prettyColors[7]:  "light-blue",
	prettyColors[8]:  "blue",
	prettyColors[9]:  "purple",
	prettyColors[10]: "pink",
	prettyColors[11]: "red-pink",
}

const ModeRainbowID = "rainbow"

type ModeRainbow struct {
	lastColor map[string]RGB

	// logging writers
	writer     *uilive.Writer
	otherLines map[string]io.Writer
}

func (mc *ModeRainbow) onSwitch(cont *controller) error {
	mc.writer = log.New()
	mc.writer.Start()

	log.FPrintln(mc.writer, log.OutputColor, fmt.Sprintf("Starting Rainbow Mode..."))
	rand.Seed(time.Now().UnixNano())
	return nil
}

func (mc *ModeRainbow) run(cont *controller) (time.Duration, error) {
	if len(mc.otherLines) == 0 {
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

	log.FPrintf(mc.writer, log.MainColor, "\t\t[Rainbow Mode]\n")

	alreadyChosen := make(map[RGB]struct{}, len(cont.devices))
	for _, device := range cont.devices {
		var color RGB
		lastColor, ok := mc.lastColor[device.DeviceID()]
		for attempts := 0; attempts < 1000; attempts++ {
			color = prettyColors[indexRandom(cont)]
			if cont.debug {
				fmt.Printf("%s: trying color %s\n", device.Name(), colorNames[color])
			}
			if !ok || (ok && !lastColor.equals(color)) {
				if _, ok = alreadyChosen[color]; ok {
					continue
				}

				break
			}
		}

		alreadyChosen[color] = struct{}{}
		cont.SetRGBAsync(device, color)
		mc.lastColor[device.DeviceID()] = color
		deviceWriter := mc.otherLines[device.DeviceID()]
		rgbStr := "["
		for _, val := range color {
			rgbStr += fmt.Sprintf("%03d, ", val)
		}
		rgbStr = rgbStr[:len(rgbStr)-2] + "]"
		log.FPrintf(deviceWriter, log.OutputColor, "\t%s (%s) - %s\n", rgbStr, colorNames[color], device.Name())
		time.Sleep(50 * time.Millisecond)
	}

	log.FPrintln(mc.writer, log.MainColor, "")

	return 1000 * time.Millisecond, nil
}

func indexRandom(cont *controller) int {
	return rand.Intn(len(prettyColors))
}

func (mc ModeRainbow) onExit(cont *controller) {
	log.FPrintln(mc.writer, log.MainColor, "Exiting Rainbow Mode...")
	mc.writer.Stop()
	//
}

func (mc ModeRainbow) isIndefinite() bool {
	return true
}

func (mc ModeRainbow) getId() string {
	return ModeRainbowID
}

const ModeExperimentID = "experiment"

type ModeExperiment struct {
	writer     *uilive.Writer
	otherLines map[string]io.Writer
}

func (mc *ModeExperiment) onSwitch(cont *controller) error {
	return nil
}

func (mc *ModeExperiment) run(cont *controller) (time.Duration, error) {
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
	for _, device := range cont.devices {
		colors := scanInputV2(mc.writer, fmt.Sprintf("Enter color for device %s", device.Name()))
		split := strings.Split(colors, " ")
		if len(split) < 3 {
			if len(split) > 0 && split[0] == "exit" {
				cont.SwitchMode(ModeCommandID)
				return 50 * time.Millisecond, nil
			}

			log.FPrintf(mc.writer, log.BadColor, "must specify colors as space-separated RGB 256 numbers e.g. 255 255 0")
			return 50 * time.Millisecond, nil
		}

		r, _ := strconv.ParseUint(split[0], 10, 8)
		g, _ := strconv.ParseUint(split[1], 10, 8)
		b, _ := strconv.ParseUint(split[2], 10, 8)
		cont.SetRGB(device, RGB{uint8(r), uint8(g), uint8(b)})
		deviceWriter, ok := mc.otherLines[device.DeviceID()]
		if ok {
			rgbStr := "["
			for _, val := range []uint64{r, g, b} {
				rgbStr += fmt.Sprintf("%03d, ", val)
			}
			rgbStr = rgbStr[:len(rgbStr)-2] + "]"
			log.FPrintf(deviceWriter, log.OutputColor, "\t%s - %s\n", rgbStr, device.Name())
		}
	}

	return 50 * time.Millisecond, nil
}

func (mc ModeExperiment) onExit(cont *controller) {
	mc.writer.Stop()
	mc.writer = nil
}

func (mc ModeExperiment) isIndefinite() bool {
	return false
}

func (mc ModeExperiment) getId() string {
	return ModeExperimentID
}
