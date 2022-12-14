package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/kungfukennyg/home-office/cync-lights/colors"
	"github.com/kungfukennyg/home-office/cync-lights/log"
	"github.com/kungfukennyg/home-office/cync-lights/optional"
	"github.com/pkg/errors"
	"github.com/unixpickle/cbyge"
)

const timeout time.Duration = 2 * time.Second

type controller struct {
	wrapped              *cbyge.Controller
	mode                 Mode
	running              bool
	debug                bool
	modes                map[string]Mode
	devices              []*cbyge.ControllerDevice
	devicesLastUpdatedAt time.Time
	userinput            chan string
	closeCh              chan<- struct{}

	lastColor map[string]colors.RGB
}

type ErrSwitchMode struct {
	modeId string
}

func (e *ErrSwitchMode) Error() string {
	return fmt.Sprintf("unrecognized mode %s", e.modeId)
}

func main() {
	args := os.Args[1:]

	debug := isDebug(args)

	if debug {
		// BaseUiLiveSingleLineTest()
		// BaseUiLiveMultiLineTest()
		// LoggerSingleLineTest()
		// LoggerMultiLineTest(2)
		// time.Sleep(10 * time.Second)
	}

	var geController *cbyge.Controller
	cachedSession := os.Getenv(CyncSession)
	if cachedSession != "" {
		sessionInfo := cbyge.SessionInfo{}
		err := json.Unmarshal([]byte(cachedSession), &sessionInfo)
		if err != nil {
			fmt.Printf("[main] couldn't unmarshal cached session info %v", sessionInfo)
			os.Exit(5)
		}
		if debug {
			fmt.Printf("[main] logging in with cached session info %v\n", cachedSession)
		}
		geController = cbyge.NewController(&sessionInfo, timeout)
	} else {
		user, pass := parseArgs(args)
		if user == "" || pass == "" {
			var ret string
			if user == "" {
				ret = "cync username"
			}
			if pass == "" {
				ret = ret + ", cync password"
			}
			fmt.Printf("[main] couldn't find %s checking env\n", ret)
			user, pass = loadEnv()
		}
		if debug {
			fmt.Printf("[main] logging in with user %v and pass <redacted>, len: %d\n", user, len(pass))
		}
		ret, err := MFALogin(user, pass)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(2)
		}
		geController = ret
	}

	c, err := newController(geController, debug)
	if err != nil {
		os.Exit(3)
	}
	err = c.SwitchMode(ModeCommandID)
	if err != nil && !errors.Is(err, &ErrSwitchMode{}) {
		fmt.Printf("%v\n", err)
		os.Exit(4)
	}
	for {
		sleepMs, err := c.run()
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(100)
		}
		if !c.running {
			os.Exit(0)
		}
		time.Sleep(sleepMs)
	}
}

const CyncUser = "CYNC_USER"
const CyncPass = "CYNC_PASS"
const CyncSession = "CYNC_SESSION"

func loadEnv() (user, pass string) {
	user = os.Getenv(CyncUser)
	pass = os.Getenv(CyncPass)

	return user, pass
}

func parseArgs(args []string) (user, pass string) {
	if len(args) < 2 {
		return "", ""
	}

	email := args[0]
	password := args[1]

	return email, password
}

func isDebug(args []string) bool {
	for _, arg := range args {
		if arg == "--debug" {
			return true
		}
	}

	return false
}

func newController(comp *cbyge.Controller, debug bool) (*controller, error) {
	c := controller{
		wrapped:   comp,
		running:   true,
		debug:     debug,
		modes:     map[string]Mode{},
		lastColor: map[string]colors.RGB{},
	}
	c.modes[ModeCommandID] = &ModeCommand{}
	c.modes[ModeRainbowID] = &ModeRainbow{}
	c.modes[ModeExperimentID] = &ModeExperiment{}
	c.modes[ModeRollID] = &ModeRoll{
		colors:     colors.BaseColors,
		otherLines: map[string]io.Writer{},
	}
	// pre-load devices
	err := c.refreshDeviceCache()
	if err != nil {
		return nil, errors.Wrap(err, "failed to refresh device cache")
	}
	return &c, nil
}

func (c *controller) run() (time.Duration, error) {
	// break indefinite modes on user input
	if c.mode.isIndefinite() {
		if c.userinput == nil {
			userInputCh, closeCh := log.ListenForInputAsync()
			c.userinput = userInputCh
			c.closeCh = closeCh
		}
	outer:
		for {
			select {
			case input, ok := <-c.userinput:
				if !ok {
					break outer
				}

				if c.debug {
					fmt.Printf("got user input %s (len %d)\n", input, len(input))
				}

				if len(input) > 1 {
					c.SwitchMode(ModeCommandID)
					return time.Millisecond, nil
				}
			case <-time.After(50 * time.Millisecond):
				break outer
			}
		}
	}

	if c.debug {
		fmt.Println("\r[controller.run] state:")
		fmt.Printf("     \rrunning: %v\n", c.running)
		fmt.Printf("     \rmode: %v - %+v\n", c.mode.getId(), c.mode)
	}
	sleepTime, err := c.mode.run(c)
	if err != nil && !errors.Is(err, &ErrSwitchMode{}) {
		return time.Second, errors.Wrapf(err, "failed to execute mode %+v", c.mode)
	}
	if c.debug {
		fmt.Printf("\r[controller.run] got sleepTime: %v\n", sleepTime)
		fmt.Printf("\r[controller.run] mode indefinite? %v\n", c.mode.isIndefinite())
	}

	return sleepTime, nil
}

// Doesn't work ):
func Login(email string, password string) (*cbyge.Controller, error) {
	comp, err := cbyge.NewControllerLogin(email, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to login")
	}

	return comp, nil
}

// gotta do this manual input login
func MFALogin(email string, password string) (*cbyge.Controller, error) {
	callback, err := cbyge.Login2FA(email, password, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to login 2fa")
	}

	mfaCode := scanInput("MFALogin", "enter 2FA code")

	sessionInfo, err := callback(mfaCode)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session info from login callback")
	}

	parsed, err := json.Marshal(sessionInfo)
	if err != nil {
		return nil, errors.New("failed to marshal session info to json")
	}
	fmt.Printf("[MFALogin] store session info in env variable '%s' for faster login: %s\n", CyncSession, string(parsed))

	return cbyge.NewController(sessionInfo, 0), nil
}

func (c *controller) PrintDevices() error {
	if c.debug {
		fmt.Printf("[PrintDevices] Printing %d devices\n", len(c.devices))
		for _, d := range c.devices {
			if d == nil {
				fmt.Println("     nil")
			} else {
				fmt.Printf("     %+v\n", *d)
			}
		}
	}

	return nil
}

func (c *controller) SetStatus(device *cbyge.ControllerDevice, status bool) {
	c.wrapped.SetDeviceStatus(device, status)
}

func (c *controller) Devices() error {
	return c.Devices()
}

func (c *controller) refreshDeviceCache() error {
	devices, err := c.wrapped.Devices()
	if err != nil {
		return errors.Wrap(err, "failed to cache devices")
	}
	c.devices = devices
	c.devicesLastUpdatedAt = time.Now()
	return nil
}

func (c *controller) SetRGBAsync(device *cbyge.ControllerDevice, color colors.RGB) error {
	if c.debug {
		fmt.Printf("[controller.SetRGBAsync] setting rgb to %+v\n", color)
	}

	c.lastColor[device.DeviceID()] = color
	return c.wrapped.SetDeviceRGB(device, color.RGBA.R, color.RGBA.G, color.RGBA.B)
}

func (c *controller) SetRGB(device *cbyge.ControllerDevice, color colors.RGB) error {
	if c.debug {
		fmt.Printf("[controller.SetRGB] setting rgb to %+v\n", color)
	}

	c.lastColor[device.DeviceID()] = color
	return c.wrapped.SetDeviceRGB(device, color.RGBA.R, color.RGBA.G, color.RGBA.B)
}

func (c *controller) SetLum(device *cbyge.ControllerDevice, lum int) error {
	if c.debug {
		fmt.Printf("[controller.SetRGB] setting lum to %+v\n", lum)
	}

	return c.wrapped.SetDeviceLum(device, lum)
}

func (c *controller) SetLumAsync(device *cbyge.ControllerDevice, lum int) error {
	if c.debug {
		fmt.Printf("[controller.SetRGB] setting lum to %+v\n", lum)
	}

	return c.wrapped.SetDeviceLumAsync(device, lum)
}

func (c *controller) SwitchMode(newMode string) error {
	mode, ok := c.modes[newMode]
	if !ok {
		return &ErrSwitchMode{
			modeId: newMode,
		}
	}
	if c.debug {
		fmt.Printf("[controller.SwitchMode] changing to mode %v\n", newMode)
	}
	c.mode = mode
	if c.mode.isIndefinite() && c.userinput != nil {
		c.closeCh <- struct{}{}
		c.closeCh = nil
		c.userinput = nil
	}
	return c.mode.onSwitch(c)
}

func (c *controller) getLastColor(device *cbyge.ControllerDevice) optional.Optional[colors.RGB] {
	rgb, ok := c.lastColor[device.DeviceID()]
	if !ok {
		return optional.Optional[colors.RGB]{}
	} else {
		return optional.WithValue(&rgb)
	}
}

func (c *controller) assignRandomColors(devices []*cbyge.ControllerDevice) map[string]colors.RGB {
	out := make(map[string]colors.RGB, len(devices))
	for _, device := range devices {
		var color colors.RGB
		// grab last color of this device
		lastColor, ok := c.lastColor[device.DeviceID()]
		// attempt to get a random color
	outer:
		for attempts := 0; attempts < 1000; attempts++ {
			color = colors.BaseColors[indexRandom(c)]
			// does this color match the device's current color?
			if ok && color == lastColor {
				continue outer
			}
			// did another device get assigned this color within this outer loop?
			for _, deviceColor := range out {
				if deviceColor == color {
					continue outer
				}
			}

			break outer
		}
		// color passed our validation, assign it
		out[device.DeviceID()] = color
		c.lastColor[device.DeviceID()] = color
	}

	return out
}

func indexRandom(cont *controller) int {
	return rand.Intn(len(colors.BaseColors))
}

func scanInput(component string, prompt string) string {
	fmt.Printf("\r[%s] %s: ", component, prompt)
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	return strings.Trim(input.Text(), "\n")
}

func scanInputV2(writer io.Writer, str string) string {
	log.FPrintln(writer, log.MainColor, str)
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	return strings.Trim(input.Text(), "\n")
}
