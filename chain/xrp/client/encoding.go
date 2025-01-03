package client

import (
    "bytes"
    "encoding/binary"
    "math/big"
    "sort"
    
    "github.com/btcsuite/btcd/btcutil/base58"
)

// XRPL field types and coding
const (
    ST_UINT16    = 1
    ST_UINT32    = 2
    ST_AMOUNT    = 6
    ST_VL        = 7
    ST_ACCOUNT   = 8
    ST_OBJECT    = 14
    ST_ARRAY     = 15
    ST_UINT8     = 16
)

// Transaction field IDs
const (
    TF_ACCOUNT       = 1
    TF_AMOUNT        = 2
    TF_DESTINATION   = 3
    TF_FEE          = 8
    TF_SEQUENCE     = 4
    TF_TYPE         = 2
    TF_SIGNINGPUB   = 3
    TF_SIGNATURE    = 4
    TF_NETWORKID    = 9
)

// Binary serialization
type FieldSorter struct {
    fieldID int
    typeID  int
    value   interface{}
}

func SerializePayment(payment *Payment) []byte {
    fields := []FieldSorter{
        {TF_TYPE, ST_UINT16, uint16(0)}, // Payment type
        {TF_ACCOUNT, ST_ACCOUNT, payment.Account},
        {TF_DESTINATION, ST_ACCOUNT, payment.Destination},
        {TF_AMOUNT, ST_AMOUNT, payment.Amount},
        {TF_FEE, ST_AMOUNT, payment.Fee},
        {TF_SEQUENCE, ST_UINT32, uint32(payment.Sequence)},
        {TF_NETWORKID, ST_UINT32, payment.NetworkID},
        {TF_SIGNINGPUB, ST_VL, payment.SigningPubKey},
    }
    
    // Sort fields by type ID, then field ID
    sort.Slice(fields, func(i, j int) bool {
        if fields[i].typeID != fields[j].typeID {
            return fields[i].typeID < fields[j].typeID
        }
        return fields[i].fieldID < fields[j].fieldID
    })
    
    var buf bytes.Buffer
    
    // Serialize each field
    for _, field := range fields {
        // Write field header
        header := (field.typeID << 4) | field.fieldID
        buf.WriteByte(byte(header))
        
        // Write field value based on type
        switch field.typeID {
        case ST_UINT16:
            binary.Write(&buf, binary.BigEndian, field.value.(uint16))
        case ST_UINT32:
            binary.Write(&buf, binary.BigEndian, field.value.(uint32))
        case ST_AMOUNT:
            amt := new(big.Int)
            amt.SetString(field.value.(string), 10)
            amtBytes := amt.Bytes()
            buf.Write(amtBytes)
        case ST_VL:
            data := field.value.([]byte)
            binary.Write(&buf, binary.BigEndian, uint16(len(data)))
            buf.Write(data)
        case ST_ACCOUNT:
            addr := field.value.(string)
            decoded := base58.Decode(addr[1:]) // Skip 'r' prefix
            buf.Write(decoded)
        }
    }
    
    return buf.Bytes()
}