package main

import (
	"bufio"
	"fmt"
	"image/color"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gosuri/uilive"
	"github.com/kungfukennyg/home-office/cync-lights/colors"
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
		log.FPrintf(outputWriter, log.OutputColor, "help, on, off, printdevices, exit, %s\n", modes)
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
	case "scenes":
		if len(args) < 2 {
		fmt.Println("[ModeCommand.run] Usage: scenes (list/add/remove)")
		return time.Second, nil
	}
		args = args[1:]
		sub := args[0]

		switch sub {
		case "list":
			
		}

	case "modes":
		fmt.Println("[ModeCommand.run] Modes:")
		for id := range cont.modes {
			fmt.Printf("\t%s\n", id)
		}
	case "exit":
		cont.running = false
	case ModeExperimentID, ModeRainbowID, ModeRollID:
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

const ModeRainbowID = "rainbow"

type ModeRainbow struct {
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

	randomColors := cont.assignRandomColors(cont.devices)
	for _, device := range cont.devices {
		color := randomColors[device.DeviceID()]
		cont.SetRGBAsync(device, color)
		deviceWriter := mc.otherLines[device.DeviceID()]
		rgbStr := "["
		for _, val := range color.GetRGB() {
			rgbStr += fmt.Sprintf("%03d, ", val)
		}
		rgbStr = rgbStr[:len(rgbStr)-2] + "]"
		log.FPrintf(deviceWriter, log.OutputColor, "| %-20s | %-20s | %-20s |\n", device.Name(), color.Name, rgbStr)
		time.Sleep(50 * time.Millisecond)
	}

	log.FPrintln(mc.writer, log.MainColor, "")

	return 1000 * time.Millisecond, nil
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
		inputStr := scanInputV2(mc.writer, fmt.Sprintf("Enter color for device %s (or 'exit' to leave)", device.Name()))
		split := strings.Split(inputStr, " ")
		if len(split) < 4 {
			if len(split) > 0 && split[0] == "exit" {
				cont.SwitchMode(ModeCommandID)
				return 50 * time.Millisecond, nil
			}

			log.FPrintf(mc.writer, log.BadColor, "must specify colors as space-separated RGBA 256 numbers e.g. 255 255 0 100")
			return 50 * time.Millisecond, nil
		}

		r, _ := strconv.ParseUint(split[0], 10, 8)
		g, _ := strconv.ParseUint(split[1], 10, 8)
		b, _ := strconv.ParseUint(split[2], 10, 8)
		a, _ := strconv.ParseUint(split[3], 10, 8)
		customColor := colors.RGB{Name: "custom", RGBA: color.RGBA{
			R: uint8(r),
			G: uint8(g),
			B: uint8(b),
			A: uint8(a),
		}}
		cont.SetRGB(device, customColor)
		cont.SetLum(device, customColor.GetLum())
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

const ModeRollID = "roll"

type ModeRoll struct {
	colorIndex int
	// logging writers
	writer     *uilive.Writer
	otherLines map[string]io.Writer
	colors     []colors.RGB
}

func (mc *ModeRoll) onSwitch(cont *controller) error {
	mc.writer = log.New()
	mc.writer.Start()

	log.FPrintln(mc.writer, log.OutputColor, fmt.Sprintf("Starting Rolling Mode..."))
	rand.Seed(time.Now().UnixNano())
	return nil
}

func (mc *ModeRoll) run(cont *controller) (time.Duration, error) {
	// setup per-device log lines
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

	log.FPrintf(mc.writer, log.MainColor, "\t\t[Rolling Mode]\n")

	for i, device := range cont.devices {
		colorIndex := mc.colorIndex + i
		if colorIndex >= len(mc.colors) {
			colorIndex = colorIndex - len(mc.colors)
		}
		if cont.debug {
			fmt.Printf("%s - color index: %d\n", device.Name(), colorIndex)
		}
		color := mc.colors[colorIndex]
		cont.SetRGB(device, color)
		deviceWriter := mc.otherLines[device.DeviceID()]
		rgbStr := "["
		for _, val := range color.GetRGB() {
			rgbStr += fmt.Sprintf("%03d, ", val)
		}
		rgbStr = rgbStr[:len(rgbStr)-2] + "]"
		log.FPrintf(deviceWriter, log.OutputColor, "\t%s (%s) - %s\n", rgbStr, color.Name, device.Name())
		time.Sleep(50 * time.Millisecond)
	}
	mc.colorIndex += 1

	log.FPrintln(mc.writer, log.MainColor, "")

	return 50 * time.Millisecond, nil
}

func (mc ModeRoll) onExit(cont *controller) {
	log.FPrintln(mc.writer, log.MainColor, "Exiting Rolling Mode...")
	mc.writer.Stop()
	//
}

func (mc ModeRoll) isIndefinite() bool {
	return true
}

func (mc ModeRoll) getId() string {
	return ModeRollID
}
