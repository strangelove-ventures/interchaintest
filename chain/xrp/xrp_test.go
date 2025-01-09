package xrp

// import (
// 	"fmt"
// 	"testing"

// 	"github.com/stretchr/testify/require"
// 	"github.com/Peersyst/xrpl-go/xrpl/websocket"
// 	"github.com/Peersyst/xrpl-go/xrpl/rpc"
// )

// func TestXrpl(t *testing.T) {
// 	client := websocket.NewClient(websocket.NewClientConfig().WithHost("ws://127.0.0.1:8001"))
// 	err := client.Connect()
// 	require.NoError(t, err)
// 	ledger, err := client.GetClosedLedger()
// 	require.NoError(t, err)
// 	fmt.Println("Ledger index (ws):", ledger.LedgerIndex.Uint32())

// 	rpcConfig, err := rpc.NewClientConfig("http://127.0.0.1:5005")
// 	require.NoError(t, err)
// 	rpcClient := rpc.NewClient(rpcConfig)
// 	ledger2, err := rpcClient.GetClosedLedger()
// 	require.NoError(t, err)
// 	fmt.Println("Ledger index (http):", ledger2.LedgerIndex.Uint32())
// }
