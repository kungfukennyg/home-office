package main

import (
	"fmt"
	"time"

	"github.com/gosuri/uilive"
)

func BaseUiLiveSingleLineTest() {
	writer := uilive.New()
	// start listening for updates and render
	writer.Start()

	for i := 0; i <= 100; i++ {
		fmt.Fprintf(writer, "Downloading.. (%d/%d) GB\n", i, 100)
		time.Sleep(time.Millisecond * 5)
	}

	fmt.Fprintln(writer, "Finished: Downloaded 100GB")
	writer.Stop() // flush and stop rendering
}

func BaseUiLiveMultiLineTest() {
	writer := uilive.New()
	writer2 := writer.Newline()
	// start listening for updates and render
	writer.Start()

	for i := 0; i <= 100; i++ {
		fmt.Fprintf(writer, "Downloading File 1.. (%d/%d) GB\n", i, 100)
		fmt.Fprintf(writer2, "Downloading File 2.. (%d/%d) GB\n", i, 100)
		time.Sleep(time.Millisecond * 5)
	}

	fmt.Fprintln(writer, "Finished: Downloaded 100GB")
	writer.Stop() // flush and stop rendering
}
