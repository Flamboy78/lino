package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tmlibs/cli"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/lino-network/lino/app"
)

// linoCmd is the entry point for this binary
var (
	linoCmd = &cobra.Command{
		Use:   "lino",
		Short: "Lino Blockchain (server)",
	}
)

// defaultOptions sets up the app_options for the
// default genesis file
func defaultOptions(args []string) (json.RawMessage, error) {
	addr, secret, err := server.GenerateCoinKey()
	if err != nil {
		return nil, err
	}
	fmt.Println("Secret phrase to access coins:")
	fmt.Println(secret)

	opts := fmt.Sprintf(`{
      "accounts": [{
        "address": "%s",
        "coins": [
          {
            "denom": "lino",
            "amount": 10000000000
          }
        ],
        "name": "Lino"

      }]
    }`, addr)
	fmt.Println("default address:", addr)
	return json.RawMessage(opts), nil
}

// generate Lino application
func generateApp(rootDir string, logger log.Logger) (abci.Application, error) {
	db, err := dbm.NewGoLevelDB("lino", rootDir)
	if err != nil {
		return nil, err
	}
	lb := app.NewLinoBlockchain(logger, db)
	return lb, nil
}

func main() {
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout)).
		With("module", "main")

	linoCmd.AddCommand(
		server.InitCmd(defaultOptions, logger),
		server.StartCmd(generateApp, logger),
		server.UnsafeResetAllCmd(logger),
		version.VersionCmd,
	)

	// prepare and add flags
	rootDir := os.ExpandEnv("$HOME/.lino")
	executor := cli.PrepareBaseCmd(linoCmd, "BC", rootDir)
	executor.Execute()
}