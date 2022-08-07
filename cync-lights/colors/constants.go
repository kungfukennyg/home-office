package colors

import "image/color"

const MaxLum uint8 = 100

type RGB struct {
	Name string
	RGBA color.RGBA
}

var BaseColors = []RGB{Red, Orange, Yellow, YellowGreen, Green, TealGreen, Teal, LightBlue, Blue, Purple, Pink, RedPink}

var (
	Red         = RGB{Name: "red", RGBA: color.RGBA{255, 0, 0, MaxLum}}
	Orange      = RGB{Name: "orange", RGBA: color.RGBA{255, 128, 0, MaxLum}}       // orange
	Yellow      = RGB{Name: "yellow", RGBA: color.RGBA{255, 255, 0, MaxLum}}       // yellow
	YellowGreen = RGB{Name: "yellow-green", RGBA: color.RGBA{128, 255, 0, MaxLum}} // yellow-green
	Green       = RGB{Name: "green", RGBA: color.RGBA{0, 255, 0, MaxLum}}          // green
	TealGreen   = RGB{Name: "teal-green", RGBA: color.RGBA{0, 255, 128, MaxLum}}   // teal-green
	Teal        = RGB{Name: "teal", RGBA: color.RGBA{0, 255, 255, MaxLum}}         // teal
	LightBlue   = RGB{Name: "light-blue", RGBA: color.RGBA{0, 128, 255, MaxLum}}   // light-blue
	Blue        = RGB{Name: "blue", RGBA: color.RGBA{0, 0, 255, MaxLum}}           // blue
	Purple      = RGB{Name: "purple", RGBA: color.RGBA{127, 0, 255, MaxLum}}       // purple
	Pink        = RGB{Name: "pink", RGBA: color.RGBA{255, 0, 255, MaxLum}}         // pink
	RedPink     = RGB{Name: "red-pink", RGBA: color.RGBA{255, 0, 127, MaxLum}}     // red-pink
)

func (r RGB) GetRGB() [3]uint8 {
	return [3]uint8{r.RGBA.R, r.RGBA.G, r.RGBA.B}
}

func (r RGB) GetLum() int {
	return int(r.RGBA.A)
}
