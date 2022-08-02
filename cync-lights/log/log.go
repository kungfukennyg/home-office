package log

import (
	"fmt"
	"io"
	"time"

	"github.com/gosuri/uilive"
)

func New() *uilive.Writer {
	writer := uilive.New()
	writer.RefreshInterval = time.Hour
	return writer
}

func FPrintf(writer io.Writer, format string, a ...any) (int, error) {
	if len(a) > 0 {
		return fmt.Fprintf(writer, format, a...)
	} else {
		return fmt.Fprintf(writer, format, a...)
	}
}

func FPrintln(writer io.Writer, ln string) (read int, err error) {
	read, err = fmt.Fprintln(writer, ln)
	if err != nil {
		return read, err
	}

	if uiliveWriter, ok := writer.(*uilive.Writer); ok {
		err = uiliveWriter.Flush()
		return read, err
	}

	return read, err
}
