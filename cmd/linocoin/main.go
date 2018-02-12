package main

import (
	"os"

	"github.com/lino-network/lino/cmd/linocoin/commands"
	"github.com/tendermint/tmlibs/cli"
)

func main() {
	rt := commands.RootCmd

	rt.AddCommand(
		commands.InitCmd,
		commands.StartCmd,
		commands.UnsafeResetAllCmd,
		commands.VersionCmd,
	)

	cmd := cli.PrepareMainCmd(rt, "BC", os.ExpandEnv("$HOME/.basecoin"))
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
