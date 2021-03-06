package text

import "github.com/muesli/termenv"

type Colo string

const (
	Ch1   Colo = "#FF0000" // red
	Ch2   Colo = "#FFFF00" // yellow
	Ch3   Colo = "#FF00FF" // fuchsia
	Clink Colo = "#6495ED" // cornflowerblue
	Ccode Colo = "#EEE8AA" // palegoldenrod
)

var colors = termenv.ColorProfile()

// Color returns the input with the given ANSI color applied
// color can be a hex string e.g. #FF0000
func Color(input string, color Colo) string {
	return termenv.String(input).Foreground(colors.Color(string(color))).String()
}
