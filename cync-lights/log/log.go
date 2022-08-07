package log

import (
	"io"
	"time"

	"github.com/fatih/color"

	"github.com/gosuri/uilive"
)

const MainColor = color.FgBlue
const OutputColor = color.FgGreen
const BadColor = color.FgRed

func New() *uilive.Writer {
	writer := uilive.New()
	writer.RefreshInterval = time.Hour
	return writer
}

func FPrintf(writer io.Writer, c color.Attribute, format string, a ...any) (int, error) {
	return color.New(c).Fprintf(writer, format, a...)
}

func FPrintln(writer io.Writer, c color.Attribute, ln string) (read int, err error) {
	read, err = color.New(c).Fprintln(writer, ln)
	if err != nil {
		return read, err
	}

	if uiliveWriter, ok := writer.(*uilive.Writer); ok {
		err = uiliveWriter.Flush()
		return read, err
	}

	return read, err
}
