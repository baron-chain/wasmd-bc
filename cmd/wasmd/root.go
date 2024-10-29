package main

import (
	"errors"
	"io"
	"os"

	rosettaCmd "cosmossdk.io/tools/rosetta/cmd"
	dbm "github.com/cometbft/cometbft-db"
	tmcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/app/params"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

const (
	defaultMinGasPrice = "0stake"
)

type CustomAppConfig struct {
	server.Config
	Wasm wasmtypes.WasmConfig `mapstructure:"wasm"`
}

// NewRootCmd creates a new root command for wasmd
func NewRootCmd() (*cobra.Command, params.EncodingConfig) {
	encodingConfig := app.MakeEncodingConfig()
	initConfig(encodingConfig)

	rootCmd := &cobra.Command{
		Use:   version.AppName,
		Short: "Wasm Daemon (server)",
		PersistentPreRunE: createPreRunE(encodingConfig),
	}

	initRootCmd(rootCmd, encodingConfig)
	return rootCmd, encodingConfig
}

func initConfig(encodingConfig params.EncodingConfig) {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(app.Bech32PrefixValAddr, app.Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(app.Bech32PrefixConsAddr, app.Bech32PrefixConsPub)
	cfg.SetAddressVerifier(wasmtypes.VerifyAddressLen())
	cfg.Seal()
}

func createPreRunE(encodingConfig params.EncodingConfig) func(*cobra.Command, []string) error {
	initClientCtx := createInitialClientContext(encodingConfig)

	return func(cmd *cobra.Command, _ []string) error {
		return setupCommandContext(cmd, initClientCtx)
	}
}

func createInitialClientContext(encodingConfig params.EncodingConfig) client.Context {
	return client.Context{}.
		WithCodec(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("")
}

func setupCommandContext(cmd *cobra.Command, initialClientCtx client.Context) error {
	cmd.SetOut(cmd.OutOrStdout())
	cmd.SetErr(cmd.ErrOrStderr())

	clientCtx, err := client.ReadPersistentCommandFlags(initialClientCtx, cmd.Flags())
	if err != nil {
		return err
	}

	clientCtx, err = config.ReadFromClientConfig(clientCtx)
	if err != nil {
		return err
	}

	if err := client.SetCmdClientContextHandler(clientCtx, cmd); err != nil {
		return err
	}

	customAppTemplate, customAppConfig := initAppConfig()
	return server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, initTendermintConfig())
}

func initTendermintConfig() *tmcfg.Config {
	return tmcfg.DefaultConfig()
}

func initAppConfig() (string, interface{}) {
	srvCfg := server.DefaultConfig()
	srvCfg.MinGasPrices = defaultMinGasPrice

	customAppConfig := CustomAppConfig{
		Config: *srvCfg,
		Wasm:   wasmtypes.DefaultWasmConfig(),
	}

	return server.DefaultConfigTemplate + wasmtypes.DefaultConfigTemplate(), customAppConfig
}

// Command Builders

func buildQueryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	addQuerySubCommands(cmd)
	return cmd
}

func addQuerySubCommands(cmd *cobra.Command) {
	cmd.AddCommand(
		authcmd.GetAccountCmd(),
		rpc.ValidatorCommand(),
		rpc.BlockCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
	)
	app.ModuleBasics.AddQueryCommands(cmd)
}

func buildTxCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	addTxSubCommands(cmd)
	return cmd
}

func addTxSubCommands(cmd *cobra.Command) {
	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetAuxToFeeCommand(),
	)
	app.ModuleBasics.AddTxCommands(cmd)
}

// App Creation and Export

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)
	wasmOpts := createWasmOptions(appOpts)

	return app.NewWasmApp(
		logger, db, traceStore, true,
		app.GetEnabledProposals(),
		appOpts,
		wasmOpts,
		baseappOptions...,
	)
}

func createWasmOptions(appOpts servertypes.AppOptions) []wasm.Option {
	if !cast.ToBool(appOpts.Get("telemetry.enabled")) {
		return nil
	}
	return []wasm.Option{wasmkeeper.WithVMCacheMetrics(prometheus.DefaultRegisterer)}
}

func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home is not set")
	}

	viperAppOpts, err := validateViperAppOpts(appOpts)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	wasmApp := createWasmAppForExport(logger, db, traceStore, height, viperAppOpts)

	if height != -1 {
		if err := wasmApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	}

	return wasmApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

func validateViperAppOpts(appOpts servertypes.AppOptions) (*viper.Viper, error) {
	viperAppOpts, ok := appOpts.(*viper.Viper)
	if !ok {
		return nil, errors.New("appOpts is not viper.Viper")
	}
	viperAppOpts.Set(server.FlagInvCheckPeriod, 1)
	return viperAppOpts, nil
}

func createWasmAppForExport(logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, appOpts servertypes.AppOptions) *app.WasmApp {
	return app.NewWasmApp(
		logger, 
		db,
		traceStore,
		height == -1,
		app.GetEnabledProposals(),
		appOpts,
		nil,
	)
}
