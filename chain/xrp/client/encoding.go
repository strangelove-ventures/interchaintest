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

// Transaction Types.
const (
	PaymentTransactionType = 0
)

// XRPL field types and coding.
const (
	stUint16  = 1
	stUint32  = 2
	stAmount  = 6
	stVl      = 7
	stAccount = 8
	stObject  = 14
	stArray   = 15
	stUint8   = 16
)

// Transaction field IDs.
const (
	tfTransactionType = 2
	tfAccount         = 1
	tfSequence        = 4
	tfFee             = 8
	tfAmount          = 1
	tfDestination     = 3
	tfSigningPub      = 3
	tfSignature       = 4
	tfNetworkID       = 1
	tfFlags           = 2
	tfMemos           = 9  // Field code for Memos array.
	tfMemo            = 10 // FIeld code for Memo object.

	// Memo field identifiers.
	tfMemoType   = 12
	tfMemoData   = 13
	tfMemoFormat = 14
)

// Binary serialization.
type FieldSorter struct {
	fieldID int
	typeID  int
	value   interface{}
}

// TODO: return error instead of panic.
func SerializePayment(payment *types.Payment, includeSig bool) ([]byte, error) {
	fields := []FieldSorter{
		{tfTransactionType, stUint16, uint16(PaymentTransactionType)},
		{tfAccount, stAccount, payment.Account},
		{tfDestination, stAccount, payment.Destination},
		{tfAmount, stAmount, payment.Amount},
		{tfFee, stAmount, payment.Fee},
		{tfSequence, stUint32, uint32(payment.Sequence)},
		{tfSigningPub, stVl, payment.SigningPubKey},
	}

	if payment.Flags != 0 {
		fields = append(fields, FieldSorter{
			fieldID: tfFlags,
			typeID:  stUint32,
			value:   payment.Flags,
		})
	}

	if includeSig {
		fields = append(fields, FieldSorter{
			fieldID: tfSignature,
			typeID:  stVl,
			value:   payment.TxnSignature,
		})
	}

	// MUST BE OMITTED for Mainnet and some test networks.
	// REQUIRED on chains whose network ID is 1025 or higher.
	if payment.NetworkID > 1024 {
		fields = append(fields, FieldSorter{
			fieldID: tfNetworkID,
			typeID:  stUint32,
			value:   payment.NetworkID,
		})
	}

	// Add Memos if present.
	if len(payment.Memos) > 0 {
		memoFields := serializeMemos(payment.Memos)
		fields = append(fields, FieldSorter{
			fieldID: tfMemos,
			typeID:  stArray,
			value:   memoFields,
		})
	}

	// Sort fields by type ID, then field ID.
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].typeID != fields[j].typeID {
			return fields[i].typeID < fields[j].typeID
		}
		return fields[i].fieldID < fields[j].fieldID
	})

	var buf bytes.Buffer

	// Write transaction prefix.
	if !includeSig {
		_ = buf.WriteByte(0x53) // 'S' for start of transaction
		_ = buf.WriteByte(0x54) // 'T' for transaction
		_ = buf.WriteByte(0x58) // 'X'
		_ = buf.WriteByte(0x00)
	}
	// fmt.Println("Transaction prefix written:", hex.EncodeToString(buf.Bytes()))
	// fmt.Println("\nField order after sorting:")
	// for i, field := range fields {
	// 	fmt.Printf("Field %d: TypeID=%d, FieldID=%d\n", i, field.typeID, field.fieldID)
	// }
	// Serialize each field.
	for _, field := range fields {
		if err := serializeField(&buf, field); err != nil {
			return nil, err
		}
	}
	// finalBytes := buf.Bytes()
	// fmt.Printf("\nFinal tx_blob: %s\n", hex.EncodeToString(finalBytes))
	return buf.Bytes(), nil
}

// Helper function to serialize memo fields.
func serializeMemos(memos []types.Memo) []FieldSorter {
	var memoFields []FieldSorter

	for _, memo := range memos {
		var memoObjFields []FieldSorter

		// Ensure at least one memo field is present.
		if memo.MemoType == "" && memo.MemoData == "" && memo.MemoFormat == "" {
			continue // Skip empty memos.
		}

		if memo.MemoType != "" {
			memoObjFields = append(memoObjFields, FieldSorter{
				fieldID: tfMemoType,
				typeID:  stVl,
				value:   memo.MemoType,
			})
		}

		if memo.MemoData != "" {
			memoObjFields = append(memoObjFields, FieldSorter{
				fieldID: tfMemoData,
				typeID:  stVl,
				value:   memo.MemoData,
			})
		}

		if memo.MemoFormat != "" {
			memoObjFields = append(memoObjFields, FieldSorter{
				fieldID: tfMemoFormat,
				typeID:  stVl,
				value:   memo.MemoFormat,
			})
		}

		// Add the object to the array.
		memoFields = append(memoFields, FieldSorter{
			fieldID: tfMemo,
			typeID:  stObject,
			value:   memoObjFields,
		})
	}

	return memoFields
}

// Helper function to serialize a single field.
func serializeField(buf *bytes.Buffer, field FieldSorter) error {
	// startPos := buf.Len()

	// Write field header
	//nolint:gocritic
	if field.typeID <= 15 && field.fieldID <= 15 {
		header := byte((field.typeID << 4) | field.fieldID)
		_ = buf.WriteByte(header)
	} else if field.typeID > 15 && field.fieldID > 15 {
		_ = buf.WriteByte(0x00)
		_ = buf.WriteByte(byte(field.typeID))
		_ = buf.WriteByte(byte(field.fieldID))
	} else if field.typeID > 15 {
		_ = buf.WriteByte(0x00 | byte(field.fieldID&0x0F))
		_ = buf.WriteByte(byte(field.typeID))
	} else {
		_ = buf.WriteByte(byte(field.typeID << 4))
		_ = buf.WriteByte(byte(field.fieldID))
	}

	// headerBytes := buf.Bytes()[startPos:]
	// fmt.Printf("\nField %d header bytes: %s\n", i, hex.EncodeToString(headerBytes))
	// fieldStartPos := buf.Len()

	// Write field value based on type.
	switch field.typeID {
	case stObject:
		objFields := field.value.([]FieldSorter)
		for _, objField := range objFields {
			if err := serializeField(buf, objField); err != nil {
				return err
			}
		}
		_ = buf.WriteByte(0xE1) // Object ending marker
	case stUint16:
		_ = binary.Write(buf, binary.BigEndian, field.value.(uint16))
	case stUint32:
		_ = binary.Write(buf, binary.BigEndian, field.value.(uint32))
	case stAmount:
		amountStr := field.value.(string)
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok {
			return fmt.Errorf("error amount serialize field, invalid amount format: %s", amountStr)
		}

		// Debug the amount encoding.
		// fmt.Printf("Encoding amount: %s\n", amountStr)

		// Create 8-byte buffer for amount.
		amtBytes := make([]byte, 8)
		// Convert to uint64 and use binary.BigEndian.
		amtUint := amount.Uint64()
		binary.BigEndian.PutUint64(amtBytes, amtUint)

		// Clear the top bit for XRP amount.
		amtBytes[0] &= 0x7F
		amtBytes[0] |= 0x40

		// Print encoded bytes for debugging.
		// fmt.Printf("Encoded amount bytes: %x\n", amtBytes)

		_, _ = buf.Write(amtBytes)
	case stVl:
		blob := field.value.(string)
		blobBz, err := hex.DecodeString(blob)
		if err != nil {
			return fmt.Errorf("error serialize VL: %v", err)
		}

		// Write length
		if len(blobBz) <= 192 {
			_ = buf.WriteByte(byte(len(blobBz)))
		} else if len(blobBz) <= 12480 {
			length := len(blobBz) - 193
			byte1 := byte((length >> 8) + 193)
			byte2 := byte(length & 0xFF)
			_ = buf.WriteByte(byte1)
			_ = buf.WriteByte(byte2)
		}
		// Debug print.
		// fmt.Printf("PublicKey length: %d bytes\n", len(pubKeyBytes))

		// Write actual bytes.
		_, _ = buf.Write(blobBz)

	case stAccount:
		addr := field.value.(string)
		if !strings.HasPrefix(addr, "r") {
			return fmt.Errorf("error serialize account, invalid account address format")
		}
		decoded := base58.Decode(addr)
		if len(decoded) != 25 {
			return fmt.Errorf("error serialize account, invalid account address length, len: %d, addr: %s", len(decoded), addr)
		}
		_ = buf.WriteByte(byte(0x14))
		_, _ = buf.Write(decoded[1:21])
	case stArray:
		// Handle memo array.
		memoFields := field.value.([]FieldSorter)
		for _, memoField := range memoFields {
			// Write each memo field using the same serialization logic.
			if err := serializeField(buf, memoField); err != nil {
				return err
			}
		}
		// Write array ending marker.
		_ = buf.WriteByte(0xF1)
	}
	// fieldBytes := buf.Bytes()[fieldStartPos:]
	// fmt.Printf("Field %d value bytes: %s\n", i, hex.EncodeToString(fieldBytes))
	return nil
}
