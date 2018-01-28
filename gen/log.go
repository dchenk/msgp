package gen

import (
	"fmt"
	"strings"

	"github.com/ttacon/chalk"
)

func infof(s string, v ...interface{}) {
	pushState(s)
	fmt.Printf(chalk.Green.Color(strings.Join(logStates, ": ")), v...)
	popState()
}

func infoln(s string) {
	pushState(s)
	fmt.Println(chalk.Green.Color(strings.Join(logStates, ": ")))
	popState()
}

func warnf(s string, v ...interface{}) {
	pushState(s)
	fmt.Printf(chalk.Yellow.Color(strings.Join(logStates, ": ")), v...)
	popState()
}

func warnln(s string) {
	pushState(s)
	fmt.Println(chalk.Yellow.Color(strings.Join(logStates, ": ")))
	popState()
}

var logStates []string

// push logging state
func pushState(s string) {
	logStates = append(logStates, s)
}

// pop logging state
func popState() {
	logStates = logStates[:len(logStates)-1]
}
