package main

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/server"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"github.com/CosmWasm/wasmd/app"
)

const (
	appEnvPrefix = "" // Empty string means no environment variable prefix
)

func main() {
	if err := run(); err != nil {
		handleError(err)
	}
}

func run() error {
	rootCmd, _ := NewRootCmd()
	return svrcmd.Execute(rootCmd, appEnvPrefix, app.DefaultNodeHome)
}

func handleError(err error) {
	exitCode := getExitCode(err)
	if exitCode > 1 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(exitCode)
}

func getExitCode(err error) int {
	switch e := err.(type) {
	case server.ErrorCode:
		return e.Code
	default:
		return 1
	}
}
