package transaction

import (
	"math"
	"strconv"

	zaddr "github.com/OpenBazaar/multiwallet/zcash/address"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/jkkgbe/open-zcash-pool/util"
	zecl "github.com/jkkgbe/zcash-light"
)

func BuildCoinbaseTxn(blockHeight uint64, poolAddress string, foundersRewardSatoshi int64, feeReward int64) ([]byte, chainhash.Hash) {
	// build input
	// blockheight script

	blockHeightAsHex := strconv.FormatUint(blockHeight, 16)

	var blockHeightSerial string
	if len(blockHeightAsHex)%2 != 0 {
		blockHeightSerial = "0" + blockHeightAsHex
	} else {
		blockHeightSerial = blockHeightAsHex
	}

	height := math.Ceil(float64(len(strconv.FormatUint(blockHeight<<1, 2))) / 8)
	lengthDiff := len(blockHeightSerial)/2 - int(height)

	for i := 0; i < lengthDiff; i++ {
		blockHeightSerial += "00"
	}

	length := "0" + strconv.FormatFloat(height, 'f', 0, 64)

	var blockHeightScript []byte

	blockHeightScript = append(blockHeightScript, util.HexToBytes(length)...)
	blockHeightScript = append(blockHeightScript, util.ReverseBuffer(util.HexToBytes(blockHeightSerial))...)
	blockHeightScript = append(blockHeightScript, []byte{0}...)
	// blockHeightScriptOzp := append(blockHeightScript, util.HexToBytes("4f5a502068747470733a2f2f6769746875622e636f6d2f4a4b4b4742452f6f70656e2d7a636173682d706f6f6c")...)

	var hash32 [32]byte
	copy(hash32[:], make([]byte, 32))

	coinbasePrevOutpoint := wire.OutPoint{
		Hash:  hash32,
		Index: 4294967295,
	}

	input := zecl.Input{
		PreviousOutPoint: coinbasePrevOutpoint,
		SignatureScript:  blockHeightScript,
		Sequence:         4294967295,
	}

	// calc which founder
	index := int(math.Floor(float64(blockHeight) / util.FoundersRewardAddressChangeInterval))

	poolAddressScriptFormat, _ := zaddr.DecodeAddress(poolAddress, &chaincfg.TestNet3Params)
	foundersAddressScriptFormat, _ := zaddr.DecodeAddress(util.TestFoundersRewardAddresses[index], &chaincfg.TestNet3Params)

	poolScript, _ := zaddr.PayToAddrScript(poolAddressScriptFormat)
	founderScript, _ := zaddr.PayToAddrScript(foundersAddressScriptFormat)

	outputPool := zecl.Output{
		Value:        1250000000 - foundersRewardSatoshi + feeReward,
		ScriptPubKey: poolScript,
	}

	outputFounders := zecl.Output{
		Value:        foundersRewardSatoshi,
		ScriptPubKey: founderScript,
	}

	transaction := zecl.Transaction{
		IsOverwinter:          true,
		Version:               4,
		VersionGroupID:        0x892F2085,
		LockTime:              0,
		ExpiryHeight:          0,
		ValueBalance:          0,
		TemporaryUnknownValue: 0,
		Inputs:                []zecl.Input{input},
		Outputs:               []zecl.Output{outputPool, outputFounders},
	}

	transactionBytes, _ := transaction.MarshalBinary()

	return transactionBytes, transaction.TxHash()
}

// import (
// 	"bytes"
// 	"encoding/binary"
// 	"io"

// 	"github.com/btcsuite/btcd/chaincfg/chainhash"
// 	"github.com/btcsuite/btcd/wire"
// )

// type Tx struct {
// 	IsOverwinter       bool
// 	Version            uint32
// 	VersionGroupID     uint32
// 	Inputs             []Input
// 	Outputs            []Output
// 	LockTime           uint32
// 	ExpiryHeight       uint32
// 	JoinSplits         []JoinSplit
// 	JoinSplitPubKey    [32]byte
// 	JoinSplitSignature [64]byte
// }

// type Input struct {
// 	PreviousOutPoint wire.OutPoint
// 	SignatureScript  []byte
// 	Sequence         uint32
// }

// type Output struct {
// 	Value        int64
// 	ScriptPubKey []byte
// }

// type JoinSplit struct {
// 	VPubOld      uint64
// 	VPubNew      uint64
// 	Anchor       [32]byte
// 	Nullifiers   [2][32]byte
// 	Commitments  [2][32]byte
// 	EphemeralKey [32]byte
// 	RandomSeed   [32]byte
// 	Macs         [2][32]byte
// 	Proof        [296]byte
// 	Ciphertexts  [2][601]byte
// }

// type countingWriter struct {
// 	io.Writer
// 	N int64
// }

// func CreateRawTransaction(inputs []Input, outputs []Output) *Tx {
// 	tx := &Tx{
// 		IsOverwinter:   false,
// 		Version:        1,
// 		VersionGroupID: 0,
// 		Inputs:         inputs,
// 		Outputs:        outputs,
// 		LockTime:       0,
// 	}

// 	if tx.IsOverwinter {
// 		tx.Version = 3
// 		tx.VersionGroupID = 0x03C48270
// 	}

// 	return tx
// }

// func (t *Tx) GetHeader() uint32 {
// 	if t.IsOverwinter {
// 		return t.Version | OverwinterFlagMask
// 	}
// 	return t.Version
// }

// func (t *Tx) TxHash() chainhash.Hash {
// 	b, _ := t.MarshalBinary()
// 	return chainhash.DoubleHashH(b)
// }

// func (t *Tx) MarshalBinary() ([]byte, error) {
// 	buf := &bytes.Buffer{}
// 	if _, err := t.WriteTo(buf); err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }

// func (t *Tx) WriteTo(w io.Writer) (n int64, err error) {
// 	counter := &countingWriter{Writer: w}
// 	for _, segment := range []func(io.Writer) error{
// 		writeField(t.GetHeader()),
// 		writeIf(t.IsOverwinter, writeField(t.VersionGroupID)),
// 		t.writeInputs,
// 		t.writeOutputs,
// 		writeField(t.LockTime),
// 		writeIf(t.IsOverwinter, writeField(t.ExpiryHeight)),
// 		writeIf(t.Version >= 2, t.writeJoinSplits),
// 		writeIf(t.Version >= 2 && len(t.JoinSplits) > 0, writeBytes(t.JoinSplitPubKey[:])),
// 		writeIf(t.Version >= 2 && len(t.JoinSplits) > 0, writeBytes(t.JoinSplitSignature[:])),
// 	} {
// 		if err := segment(counter); err != nil {
// 			return counter.N, err
// 		}
// 	}
// 	return counter.N, nil
// }

// func writeField(v interface{}) func(w io.Writer) error {
// 	return func(w io.Writer) error {
// 		return binary.Write(w, binary.LittleEndian, v)
// 	}
// }

// func writeIf(pred bool, f func(w io.Writer) error) func(w io.Writer) error {
// 	if pred {
// 		return f
// 	}
// 	return func(w io.Writer) error { return nil }
// }

// func writeBytes(v []byte) func(w io.Writer) error {
// 	return func(w io.Writer) error {
// 		_, err := w.Write(v)
// 		return err
// 	}
// }
