package benchmarks

import (
	"encoding/json"
	"testing"
	"time"

	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb/opt"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

type (
	benchmarkTestCase struct {
		db          func(*testing.B) dbm.DB
		txBuilder   func(*testing.B, *AppInfo) []sdk.Tx
		blockSize   int
		numAccounts int
	}

	transferMsg struct {
		Recipient string `json:"recipient"`
		Amount    uint64 `json:"amount"`
	}

	cw20ExecMsg struct {
		Transfer *transferMsg `json:"transfer,omitempty"`
	}
)

const (
	defaultBlockSize     = 20
	defaultNumAccounts   = 50
	largeNumAccounts    = 8000
	veryLargeNumAccounts = 80000
	hugeBlockSize       = 1000
	defaultTransferAmount = 100
	defaultCw20Amount    = 765
)

var benchmarkTestCases = map[string]benchmarkTestCase{
	"basic send - memdb": {
		db:          buildMemDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(bankSendMsg),
		numAccounts: defaultNumAccounts,
	},
	"cw20 transfer - memdb": {
		db:          buildMemDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(cw20TransferMsg),
		numAccounts: defaultNumAccounts,
	},
	"basic send - leveldb": {
		db:          buildLevelDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(bankSendMsg),
		numAccounts: defaultNumAccounts,
	},
	"cw20 transfer - leveldb": {
		db:          buildLevelDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(cw20TransferMsg),
		numAccounts: defaultNumAccounts,
	},
	"basic send - leveldb - 8k accounts": {
		db:          buildLevelDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(bankSendMsg),
		numAccounts: largeNumAccounts,
	},
	"cw20 transfer - leveldb - 8k accounts": {
		db:          buildLevelDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(cw20TransferMsg),
		numAccounts: largeNumAccounts,
	},
	"basic send - leveldb - 8k accounts - huge blocks": {
		db:          buildLevelDB,
		blockSize:   hugeBlockSize,
		txBuilder:   buildTxFromMsg(bankSendMsg),
		numAccounts: largeNumAccounts,
	},
	"cw20 transfer - leveldb - 8k accounts - huge blocks": {
		db:          buildLevelDB,
		blockSize:   hugeBlockSize,
		txBuilder:   buildTxFromMsg(cw20TransferMsg),
		numAccounts: largeNumAccounts,
	},
	"basic send - leveldb - 80k accounts": {
		db:          buildLevelDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(bankSendMsg),
		numAccounts: veryLargeNumAccounts,
	},
	"cw20 transfer - leveldb - 80k accounts": {
		db:          buildLevelDB,
		blockSize:   defaultBlockSize,
		txBuilder:   buildTxFromMsg(cw20TransferMsg),
		numAccounts: veryLargeNumAccounts,
	},
}

func BenchmarkTxSending(b *testing.B) {
	for name, tc := range benchmarkTestCases {
		b.Run(name, func(b *testing.B) {
			runBenchmarkTest(b, tc)
		})
	}
}

func runBenchmarkTest(b *testing.B, tc benchmarkTestCase) {
	db := tc.db(b)
	defer db.Close()

	appInfo := InitializeWasmApp(b, db, tc.numAccounts)
	txs := tc.txBuilder(b, &appInfo)

	height := int64(3)
	txEncoder := appInfo.TxConfig.TxEncoder()

	b.ResetTimer()

	for i := 0; i < b.N/tc.blockSize; i++ {
		processBlock(b, &appInfo, txs[i*tc.blockSize:(i+1)*tc.blockSize], height, txEncoder)
		height++
	}
}

func processBlock(b *testing.B, appInfo *AppInfo, txs []sdk.Tx, height int64, txEncoder sdk.TxEncoder) {
	appInfo.App.BeginBlock(abci.RequestBeginBlock{
		Header: tmproto.Header{
			Height: height,
			Time:   time.Now(),
		},
	})

	for _, tx := range txs {
		bz, err := txEncoder(tx)
		require.NoError(b, err)

		checkTxResponse := appInfo.App.CheckTx(abci.RequestCheckTx{
			Tx:   bz,
			Type: abci.CheckTxType_New,
		})
		require.True(b, checkTxResponse.IsOK())

		deliverTxResponse := appInfo.App.DeliverTx(abci.RequestDeliverTx{Tx: bz})
		require.True(b, deliverTxResponse.IsOK())
	}

	appInfo.App.EndBlock(abci.RequestEndBlock{Height: height})
	appInfo.App.Commit()
}

func bankSendMsg(info *AppInfo) ([]sdk.Msg, error) {
	recipient := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	coins := sdk.NewCoins(sdk.NewInt64Coin(info.Denom, defaultTransferAmount))
	
	return []sdk.Msg{
		banktypes.NewMsgSend(info.MinterAddr, recipient, coins),
	}, nil
}

func cw20TransferMsg(info *AppInfo) ([]sdk.Msg, error) {
	recipient := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	
	transfer := cw20ExecMsg{
		Transfer: &transferMsg{
			Recipient: recipient.String(),
			Amount:    defaultCw20Amount,
		},
	}
	
	transferBz, err := json.Marshal(transfer)
	if err != nil {
		return nil, err
	}

	return []sdk.Msg{
		&wasmtypes.MsgExecuteContract{
			Sender:   info.MinterAddr.String(),
			Contract: info.ContractAddr,
			Msg:      transferBz,
		},
	}, nil
}

func buildTxFromMsg(builder func(info *AppInfo) ([]sdk.Msg, error)) func(b *testing.B, info *AppInfo) []sdk.Tx {
	return func(b *testing.B, info *AppInfo) []sdk.Tx {
		return GenSequenceOfTxs(b, info, builder, b.N)
	}
}

func buildMemDB(_ *testing.B) dbm.DB {
	return dbm.NewMemDB()
}

func buildLevelDB(b *testing.B) dbm.DB {
	levelDB, err := dbm.NewGoLevelDBWithOpts(
		"testing",
		b.TempDir(),
		&opt.Options{BlockCacher: opt.NoCacher},
	)
	require.NoError(b, err)
	return levelDB
}
