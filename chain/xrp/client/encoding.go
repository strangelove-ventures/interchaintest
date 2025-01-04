package client

import (
	"bytes"
	"encoding/binary"
    "encoding/json"
    "encoding/hex"
	"math/big"
	"sort"
    "strings"
    "fmt"

	addresscodec "github.com/strangelove-ventures/interchaintest/v8/chain/xrp/address-codec"
    "github.com/xyield/xrpl-go/binary-codec"
    //"github.com/xyield/xrpl-go/model/transactions"
   // "github.com/xyield/xrpl-go/model/transactions/types"
)

// Transaction Types
const (
	PAYMENT_TRANSACTION_TYPE = 0
)

// XRPL field types and coding
const (
	ST_UINT16  = 1
	ST_UINT32  = 2
	ST_AMOUNT  = 6
	ST_VL      = 7
	ST_ACCOUNT = 8
	ST_OBJECT  = 14
	ST_ARRAY   = 15
	ST_UINT8   = 16
)

// Transaction field IDs
const (
	TF_TRANSACTION_TYPE = 2
	TF_ACCOUNT          = 1
	TF_SEQUENCE         = 4
	TF_FEE              = 8
	TF_AMOUNT           = 1 //2
	TF_DESTINATION      = 3
	TF_SIGNINGPUB       = 3 //TF_PUBLIC_KEY
	TF_SIGNATURE   = 4
	TF_NETWORKID   = 1
    TF_FLAGS = 2
)

// Binary serialization
type FieldSorter struct {
	fieldID int
	typeID  int
	value   interface{}
}

// // Define field order priority
// var fieldOrderPriority = map[int]int{
//     // TypeID_FieldID : priority (lower = earlier in sort)
//     0x12: 1,  // TransactionType
//     0x81: 2,  // Account 
//     0x24: 3,  // Sequence
//     0x68: 4,  // Fee
//     0x73: 5,  // SigningPubKey
//     0x62: 6,  // Amount
//     0x83: 7,  // Destination
// }

// // Define explicit field order
// var fieldOrder = map[string]int{
//     "TransactionType": 1,
//     "Account": 2,
//     "Amount": 3,
//     "Destination": 4,
//     "Fee": 5,
//     "Sequence": 6,
//     "SigningPubKey": 7,
// }

// func getFieldName(field FieldSorter) string {
//     key := (field.typeID << 8) | field.fieldID  // Create unique key combining both IDs
    
//     switch key {
//     case (ST_UINT16 << 8) | TF_TRANSACTION_TYPE:
//         return "TransactionType"
//     case (ST_ACCOUNT << 8) | TF_ACCOUNT:
//         return "Account"
//     case (ST_AMOUNT << 8) | TF_AMOUNT:
//         return "Amount"
//     case (ST_ACCOUNT << 8) | TF_DESTINATION:
//         return "Destination"
//     case (ST_AMOUNT << 8) | TF_FEE:
//         return "Fee"
//     case (ST_UINT32 << 8) | TF_SEQUENCE:
//         return "Sequence"
//     case (ST_VL << 8) | TF_SIGNINGPUB:
//         return "SigningPubKey"
//     default:
//         return fmt.Sprintf("Unknown_%d_%d", field.typeID, field.fieldID)
//     }
// }
var fieldOrder = map[uint16]int{
    // (ST_UINT16 << 8) | TF_TRANSACTION_TYPE:      1,  // TransactionType
    // (ST_ACCOUNT << 8) | TF_ACCOUNT:   3,  // Account
    // (ST_UINT32 << 8) | TF_SEQUENCE:   4,  // Sequence
    // (ST_AMOUNT << 8) | TF_FEE:        5,  // Fee
    // (ST_VL << 8) | TF_SIGNINGPUB:     6,  // SigningPubKey
    // (ST_AMOUNT << 8) | TF_AMOUNT:     7,  // Amount
    // (ST_ACCOUNT << 8) | TF_DESTINATION: 8, // Destination
    // (ST_VL << 8) | TF_SIGNATURE:      9,  // TxnSignature comes last
    (ST_UINT16 << 8) | TF_TRANSACTION_TYPE:      1,  // TransactionType
    (ST_ACCOUNT << 8) | TF_ACCOUNT:   10,  // Account
    (ST_UINT32 << 8) | TF_SEQUENCE:   5,  // Sequence
    (ST_AMOUNT << 8) | TF_FEE:        7,  // Fee
    (ST_VL << 8) | TF_SIGNINGPUB:     8,  // SigningPubKey
    (ST_AMOUNT << 8) | TF_AMOUNT:     6,  // Amount
    (ST_ACCOUNT << 8) | TF_DESTINATION: 11, // Destination
    (ST_VL << 8) | TF_SIGNATURE:      9,//9,  // TxnSignature comes last
    (ST_UINT32 << 8) | TF_NETWORKID:      3,//9,  // TxnSignature comes last
    (ST_UINT32 << 8) | TF_FLAGS:      4,//9,  // TxnSignature comes last
}


// TODO: return error instead of panic
func SerializePayment(payment *Payment, includeSig bool) []byte {
    // payment2 := transactions.Payment{
    //     BaseTx: transactions.BaseTx{
    //         Account: types.Address(payment.Account),
    //         Fee: types.XRPCurrencyAmount(10),
    //         Sequence: uint(payment.Sequence),
    //         SigningPubKey: payment.SigningPubKey,
    //         TransactionType: transactions.PaymentTx,
    //     },
    //     Destination: types.Address(payment.Destination),
    //     Amount: types.XRPCurrencyAmount(200000000),
    // }
    // if includeSig {
    //     payment2.BaseTx.TxnSignature = payment.TxnSignature
    // }

    jsonBz, err := json.Marshal(payment)
    if err != nil {
        panic(err)
    }
    fmt.Println("jsonBz:", string(jsonBz))

    var jsonMap map[string]any
    err = json.Unmarshal(jsonBz, &jsonMap)
    if err != nil {
        panic(err)
    }

    jsonMap["Sequence"] = payment.Sequence
    //jsonMap["Expiration"] = 595640108
    //jsonMap["Flags"] = 524288

    // amount, err := types.XRPCurrencyAmount(200000000).MarshalJSON()
    // if err != nil {
    //     panic(err)
    // }

    // jsonMap["Amount"] = amount

    encodedPaymentHex, err := binarycodec.EncodeForSigning(jsonMap)
    if err != nil {
        panic(err)
    }

    encodedPayment, err := hex.DecodeString(encodedPaymentHex)
    if err != nil {
        panic(err)
    }
    prefix := []byte{0x53, 0x54, 0x58}
    encodedPayment = append(prefix, encodedPayment[4:]...)


    return encodedPayment
}

// TODO: return error instead of panic
func SerializePayment2(payment *Payment, includeSig bool) []byte {
	fields := []FieldSorter{
		{TF_TRANSACTION_TYPE, ST_UINT16, uint16(PAYMENT_TRANSACTION_TYPE)},
		{TF_ACCOUNT, ST_ACCOUNT, payment.Account},
		{TF_DESTINATION, ST_ACCOUNT, payment.Destination},
		{TF_AMOUNT, ST_AMOUNT, payment.Amount},
		{TF_FEE, ST_AMOUNT, payment.Fee},
		{TF_SEQUENCE, ST_UINT32, uint32(payment.Sequence)},
		{TF_NETWORKID, ST_UINT32, payment.NetworkID},
		{TF_SIGNINGPUB, ST_VL, payment.SigningPubKey},
        {TF_FLAGS, ST_UINT32, payment.Flags},
	}

    if includeSig {
        fields = append(fields, FieldSorter{
            fieldID: TF_SIGNATURE,
            typeID: ST_VL,
            value: payment.TxnSignature,
        })
    }

	// // // // Sort fields by type ID, then field ID
	// // // // sort.Slice(fields, func(i, j int) bool {
	// // // // 	if fields[i].typeID != fields[j].typeID {
	// // // // 		return fields[i].typeID < fields[j].typeID
	// // // // 	}
	// // // // 	return fields[i].fieldID < fields[j].fieldID
	// // // // })
    // // // sort.Slice(fields, func(i, j int) bool {
    // // //     iKey := (fields[i].typeID << 4) | fields[i].fieldID
    // // //     jKey := (fields[j].typeID << 4) | fields[j].fieldID
    // // //     iPriority := fieldOrderPriority[iKey]
    // // //     jPriority := fieldOrderPriority[jKey]
    // // //     return iPriority < jPriority
    // // // })
    // // sort.Slice(fields, func(i, j int) bool {
    // //     // TransactionType must always be first
    // //     if fields[i].fieldID == TF_TYPE {
    // //         return true
    // //     }
    // //     if fields[j].fieldID == TF_TYPE {
    // //         return false
    // //     }
        
    // //     // Then Account
    // //     if fields[i].fieldID == TF_ACCOUNT && fields[j].fieldID != TF_TYPE {
    // //         return true
    // //     }
    // //     if fields[j].fieldID == TF_ACCOUNT && fields[i].fieldID != TF_TYPE {
    // //         return false
    // //     }
        
    // //     // For other fields, sort by type then field ID
    // //     if fields[i].typeID != fields[j].typeID {
    // //         return fields[i].typeID < fields[j].typeID
    // //     }
    // //     return fields[i].fieldID < fields[j].fieldID
    // // })
    // sort.Slice(fields, func(i, j int) bool {
    //     iName := getFieldName(fields[i])
    //     jName := getFieldName(fields[j])
    //     return fieldOrder[iName] < fieldOrder[jName]
    // })
    sort.Slice(fields, func(i, j int) bool {
        iKey := uint16(fields[i].typeID<<8) | uint16(fields[i].fieldID)
        jKey := uint16(fields[j].typeID<<8) | uint16(fields[j].fieldID)
        
        iPriority := fieldOrder[iKey]
        jPriority := fieldOrder[jKey]
        
        if iPriority == 0 || jPriority == 0 {
            fmt.Printf("Warning: Unknown field type-id combination: %d-%d or %d-%d\n", 
                fields[i].typeID, fields[i].fieldID, fields[j].typeID, fields[j].fieldID)
        }
        
        return iPriority < jPriority
    })

	var buf bytes.Buffer

	// Write transaction prefix
    if !includeSig {
        buf.WriteByte(0x53) // 'S' for start of transaction
        buf.WriteByte(0x54) // 'T' for transaction
        buf.WriteByte(0x58) // 'X'
        buf.WriteByte(0x00)
    }
    fmt.Println("Transaction prefix written:", hex.EncodeToString(buf.Bytes()))
    fmt.Println("\nField order after sorting:")
    for i, field := range fields {
        fmt.Printf("Field %d: TypeID=%d, FieldID=%d\n", i, field.typeID, field.fieldID)
    }
	// Serialize each field
	for i, field := range fields {
        startPos := buf.Len()
		// Write field header
		if field.typeID <= 15 && field.fieldID <= 15 {
			// Case 1: Both small - single byte header
			header := byte((field.typeID << 4) | field.fieldID)
			buf.WriteByte(header)
		} else if field.typeID > 15 && field.fieldID > 15 {
			// Case 2: Both large - two bytes with special encoding
			buf.WriteByte(0x00) // First byte is 0 to indicate both are large
			buf.WriteByte(byte(field.typeID))
			buf.WriteByte(byte(field.fieldID))
		} else if field.typeID > 15 {
			// Case 3: Large TypeID, small FieldID
			buf.WriteByte(0x00 | byte(field.fieldID & 0x0F))
			buf.WriteByte(byte(field.typeID))
		} else {
			// Case 4: Small TypeID, large FieldID
			buf.WriteByte(byte(field.typeID << 4))
			buf.WriteByte(byte(field.fieldID))
		}
		// // Write field header
		// header := (field.typeID << 4) | field.fieldID
		// buf.WriteByte(byte(header))
        headerBytes := buf.Bytes()[startPos:]
        fmt.Printf("\nField %d header bytes: %s\n", i, hex.EncodeToString(headerBytes))
        fieldStartPos := buf.Len()

		// Write field value based on type
		switch field.typeID {
		case ST_UINT16:
			binary.Write(&buf, binary.BigEndian, field.value.(uint16))
		case ST_UINT32:
			binary.Write(&buf, binary.BigEndian, field.value.(uint32))
		case ST_AMOUNT:
            amountStr := field.value.(string)
            amount, ok := new(big.Int).SetString(amountStr, 10)
            if !ok {
                panic(fmt.Sprintf("invalid amount format: %s", amountStr))
            }
            
            // Debug the amount encoding
            fmt.Printf("Encoding amount: %s\n", amountStr)
            
            // Create 8-byte buffer for amount
            amtBytes := make([]byte, 8)
            // Convert to uint64 and use binary.BigEndian
            amtUint := amount.Uint64()
            binary.BigEndian.PutUint64(amtBytes, amtUint)
            
            // Clear the top bit for XRP amount
            amtBytes[0] &= 0x7F
            amtBytes[0] |= 0x40
            
            // Print encoded bytes for debugging
            fmt.Printf("Encoded amount bytes: %x\n", amtBytes)
            
            buf.Write(amtBytes)
			// amountStr := field.value.(string)
			// amt, ok := new(big.Int).SetString(amountStr, 10)
			// if !ok {
			// 	panic("invalid amount format")
			// }

			// // Create 8-byte buffer for XRP amount
			// amtBytes := make([]byte, 8)
			// // Convert to big-endian
			// amt64 := amt.Uint64()
			// binary.BigEndian.PutUint64(amtBytes, amt64)
			// // Clear the top bit for XRP amounts (not issued currency)
			// amtBytes[0] &= 0x7F
			// buf.Write(amtBytes)
			// // amt := new(big.Int)
			// // amt.SetString(field.value.(string), 10)
			// // amtBytes := amt.Bytes()
			// // buf.Write(amtBytes)
		case ST_VL:
            pubKeyHex := field.value.(string)
            // Decode hex string to bytes instead of using it as ASCII
            pubKeyBytes, err := hex.DecodeString(pubKeyHex)
            if err != nil {
                panic(fmt.Sprintf("invalid hex in SigningPubKey: %v", err))
            }
            
            // Write length
            if len(pubKeyBytes) <= 192 {
                buf.WriteByte(byte(len(pubKeyBytes)))
            } else if len(pubKeyBytes) <= 12480 {
                length := len(pubKeyBytes) - 193
                byte1 := byte((length >> 8) + 193)
                byte2 := byte(length & 0xFF)
                buf.WriteByte(byte1)
                buf.WriteByte(byte2)
            }
            // Debug print
            fmt.Printf("PublicKey length: %d bytes\n", len(pubKeyBytes))
            
            // Write actual bytes
            buf.Write(pubKeyBytes)
			// data := []byte(field.value.(string))
			// // Write variable length
			// if len(data) <= 192 {
			// 	buf.WriteByte(byte(len(data)))
			// } else if len(data) <= 12480 {
			// 	length := len(data) - 193
			// 	byte1 := byte((length >> 8) + 193)
			// 	byte2 := byte(length & 0xFF)
			// 	buf.WriteByte(byte1)
			// 	buf.WriteByte(byte2)
			// } else {
			// 	panic("data too long for variable length encoding")
			// }
			// buf.Write(data)
			// // data := field.value.(string)
			// // //data := field.value.([]byte)
			// // binary.Write(&buf, binary.BigEndian, uint16(len(data)))
			// // buf.Write([]byte(data))
		case ST_ACCOUNT:
			addr := field.value.(string)
			if !strings.HasPrefix(addr, "r") {
				panic("invalid account address format")
			}
            fmt.Println("serialize addr:", addr)
			decoded := addresscodec.DecodeBase58(addr) // Skip 'r' prefix
			if len(decoded) != 25 {
				panic(fmt.Sprintf("invalid account address length, len: %d, addr: %s", len(decoded), addr))
			}
            buf.WriteByte(byte(0x14))
			buf.Write(decoded[1:21])
			// addr := field.value.(string)
			// decoded := addresscodec.DecodeBase58(addr[1:]) // Skip 'r' prefix
			// //decoded := base58.Decode(addr[1:]) // Skip 'r' prefix
			// buf.Write(decoded)
		}
        fieldBytes := buf.Bytes()[fieldStartPos:]
        fmt.Printf("Field %d value bytes: %s\n", i, hex.EncodeToString(fieldBytes))
	}
    finalBytes := buf.Bytes()
    fmt.Printf("\nFinal tx_blob: %s\n", hex.EncodeToString(finalBytes))
	return buf.Bytes()
}
