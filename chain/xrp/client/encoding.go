package client

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client/base58"
	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client/types"
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
	TF_AMOUNT           = 1
	TF_DESTINATION      = 3
	TF_SIGNINGPUB       = 3
	TF_SIGNATURE        = 4
	TF_NETWORKID        = 1
	TF_FLAGS            = 2
	TF_MEMOS            = 9  // Field code for Memos array
	TF_MEMO             = 10 // FIeld code for Memo object

	// Memo field identifiers
	TF_MEMO_TYPE   = 12
	TF_MEMO_DATA   = 13
	TF_MEMO_FORMAT = 14
)

// Binary serialization
type FieldSorter struct {
	fieldID int
	typeID  int
	value   interface{}
}

// TODO: return error instead of panic
func SerializePayment(payment *types.Payment, includeSig bool) ([]byte, error) {
	fields := []FieldSorter{
		{TF_TRANSACTION_TYPE, ST_UINT16, uint16(PAYMENT_TRANSACTION_TYPE)},
		{TF_ACCOUNT, ST_ACCOUNT, payment.Account},
		{TF_DESTINATION, ST_ACCOUNT, payment.Destination},
		{TF_AMOUNT, ST_AMOUNT, payment.Amount},
		{TF_FEE, ST_AMOUNT, payment.Fee},
		{TF_SEQUENCE, ST_UINT32, uint32(payment.Sequence)},
		{TF_SIGNINGPUB, ST_VL, payment.SigningPubKey},
	}

	if payment.Flags != 0 {
		fields = append(fields, FieldSorter{
			fieldID: TF_FLAGS,
			typeID:  ST_UINT32,
			value:   payment.Flags,
		})
	}

	if includeSig {
		fields = append(fields, FieldSorter{
			fieldID: TF_SIGNATURE,
			typeID:  ST_VL,
			value:   payment.TxnSignature,
		})
	}

	// MUST BE OMITTED for Mainnet and some test networks.
	// REQUIRED on chains whose network ID is 1025 or higher.
	if payment.NetworkID > 1024 {
		fields = append(fields, FieldSorter{
			fieldID: TF_NETWORKID,
			typeID:  ST_UINT32,
			value:   payment.NetworkID,
		})
	}

	// Add Memos if present
	if len(payment.Memos) > 0 {
		memoFields := serializeMemos(payment.Memos)
		fields = append(fields, FieldSorter{
			fieldID: TF_MEMOS,
			typeID:  ST_ARRAY,
			value:   memoFields,
		})
	}

	// Sort fields by type ID, then field ID
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].typeID != fields[j].typeID {
			return fields[i].typeID < fields[j].typeID
		}
		return fields[i].fieldID < fields[j].fieldID
	})

	var buf bytes.Buffer

	// Write transaction prefix
	if !includeSig {
		buf.WriteByte(0x53) // 'S' for start of transaction
		buf.WriteByte(0x54) // 'T' for transaction
		buf.WriteByte(0x58) // 'X'
		buf.WriteByte(0x00)
	}
	// fmt.Println("Transaction prefix written:", hex.EncodeToString(buf.Bytes()))
	// fmt.Println("\nField order after sorting:")
	// for i, field := range fields {
	// 	fmt.Printf("Field %d: TypeID=%d, FieldID=%d\n", i, field.typeID, field.fieldID)
	// }
	// Serialize each field
	for _, field := range fields {
		serializeField(&buf, field)
	}
	// finalBytes := buf.Bytes()
	// fmt.Printf("\nFinal tx_blob: %s\n", hex.EncodeToString(finalBytes))
	return buf.Bytes(), nil
}

// Helper function to serialize memo fields
func serializeMemos(memos []types.Memo) []FieldSorter {
	var memoFields []FieldSorter

	for _, memo := range memos {
		var memoObjFields []FieldSorter

		// Ensure at least one memo field is present
		if memo.MemoType == "" && memo.MemoData == "" && memo.MemoFormat == "" {
			continue // Skip empty memos
		}

		if memo.MemoType != "" {
			memoObjFields = append(memoObjFields, FieldSorter{
				fieldID: TF_MEMO_TYPE,
				typeID:  ST_VL,
				value:   memo.MemoType,
			})
		}

		if memo.MemoData != "" {
			memoObjFields = append(memoObjFields, FieldSorter{
				fieldID: TF_MEMO_DATA,
				typeID:  ST_VL,
				value:   memo.MemoData,
			})
		}

		if memo.MemoFormat != "" {
			memoObjFields = append(memoObjFields, FieldSorter{
				fieldID: TF_MEMO_FORMAT,
				typeID:  ST_VL,
				value:   memo.MemoFormat,
			})
		}

		// Add the object to the array
		memoFields = append(memoFields, FieldSorter{
			fieldID: TF_MEMO,
			typeID:  ST_OBJECT,
			value:   memoObjFields,
		})
	}

	return memoFields
}

// Helper function to serialize a single field
func serializeField(buf *bytes.Buffer, field FieldSorter) error {
	// startPos := buf.Len()

	// Write field header
	if field.typeID <= 15 && field.fieldID <= 15 {
		header := byte((field.typeID << 4) | field.fieldID)
		buf.WriteByte(header)
	} else if field.typeID > 15 && field.fieldID > 15 {
		buf.WriteByte(0x00)
		buf.WriteByte(byte(field.typeID))
		buf.WriteByte(byte(field.fieldID))
	} else if field.typeID > 15 {
		buf.WriteByte(0x00 | byte(field.fieldID&0x0F))
		buf.WriteByte(byte(field.typeID))
	} else {
		buf.WriteByte(byte(field.typeID << 4))
		buf.WriteByte(byte(field.fieldID))
	}

	// headerBytes := buf.Bytes()[startPos:]
	// fmt.Printf("\nField %d header bytes: %s\n", i, hex.EncodeToString(headerBytes))
	// fieldStartPos := buf.Len()

	// Write field value based on type
	switch field.typeID {
	case ST_OBJECT:
		objFields := field.value.([]FieldSorter)
		for _, objField := range objFields {
			if err := serializeField(buf, objField); err != nil {
				return err
			}
		}
		buf.WriteByte(0xE1) // Object ending marker
	case ST_UINT16:
		binary.Write(buf, binary.BigEndian, field.value.(uint16))
	case ST_UINT32:
		binary.Write(buf, binary.BigEndian, field.value.(uint32))
	case ST_AMOUNT:
		amountStr := field.value.(string)
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok {
			return fmt.Errorf("error amount serialize field, invalid amount format: %s", amountStr)
		}

		// Debug the amount encoding
		// fmt.Printf("Encoding amount: %s\n", amountStr)

		// Create 8-byte buffer for amount
		amtBytes := make([]byte, 8)
		// Convert to uint64 and use binary.BigEndian
		amtUint := amount.Uint64()
		binary.BigEndian.PutUint64(amtBytes, amtUint)

		// Clear the top bit for XRP amount
		amtBytes[0] &= 0x7F
		amtBytes[0] |= 0x40

		// Print encoded bytes for debugging
		// fmt.Printf("Encoded amount bytes: %x\n", amtBytes)

		buf.Write(amtBytes)
	case ST_VL:
		blob := field.value.(string)
		blobBz, err := hex.DecodeString(blob)
		if err != nil {
			return fmt.Errorf("error serialize VL: %v", err)
		}

		// Write length
		if len(blobBz) <= 192 {
			buf.WriteByte(byte(len(blobBz)))
		} else if len(blobBz) <= 12480 {
			length := len(blobBz) - 193
			byte1 := byte((length >> 8) + 193)
			byte2 := byte(length & 0xFF)
			buf.WriteByte(byte1)
			buf.WriteByte(byte2)
		}
		// Debug print
		// fmt.Printf("PublicKey length: %d bytes\n", len(pubKeyBytes))

		// Write actual bytes
		buf.Write(blobBz)

	case ST_ACCOUNT:
		addr := field.value.(string)
		if !strings.HasPrefix(addr, "r") {
			return fmt.Errorf("error serialize account, invalid account address format")
		}
		decoded := base58.Decode(addr)
		if len(decoded) != 25 {
			return fmt.Errorf("error serialize account, invalid account address length, len: %d, addr: %s", len(decoded), addr)
		}
		buf.WriteByte(byte(0x14))
		buf.Write(decoded[1:21])
	case ST_ARRAY:
		// Handle memo array
		memoFields := field.value.([]FieldSorter)
		for _, memoField := range memoFields {
			// Write each memo field using the same serialization logic
			if err := serializeField(buf, memoField); err != nil {
				return err
			}
		}
		// Write array ending marker
		buf.WriteByte(0xF1)
	}
	// fieldBytes := buf.Bytes()[fieldStartPos:]
	// fmt.Printf("Field %d value bytes: %s\n", i, hex.EncodeToString(fieldBytes))
	return nil
}
