package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var (
	blueArrow  = color.New(color.FgBlue, color.Bold).Sprint("==>")
	greenCheck = color.New(color.FgGreen, color.Bold).Sprint("✓")
	yellowBang = color.New(color.FgYellow, color.Bold).Sprint("!")
	grayBar    = color.New(color.FgWhite).Sprint(strings.Repeat("─", 64))
)

func Log(s string) {
	fmt.Printf("%s %s\n", blueArrow, s)
}

func Ok(s string) {
	fmt.Printf("%s   %s\n", greenCheck, s)
}

func Warn(s string) {
	fmt.Printf("%s   %s\n", yellowBang, s)
}

func Success(s string) {
	color.New(color.FgGreen, color.Bold).Println(s)
}

func Bar() {
	fmt.Println(grayBar)
}
