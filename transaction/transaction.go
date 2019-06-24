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

func BuildCoinbaseTxn(blockHeight int64, poolAddress string, foundersRewardZatoshi int64, feeReward int64) ([]byte, chainhash.Hash) {
	blockHeightAsHex := strconv.FormatInt(blockHeight, 16)

	var blockHeightSerial string

	if len(blockHeightAsHex)%2 != 0 {
		blockHeightSerial = "0" + blockHeightAsHex
	} else {
		blockHeightSerial = blockHeightAsHex
	}

	height := math.Ceil(float64(len(strconv.FormatInt(blockHeight<<1, 2))) / 8)
	lengthDiff := len(blockHeightSerial)/2 - int(height)

	for i := 0; i < lengthDiff; i++ {
		blockHeightSerial += "00"
	}

	length := "0" + strconv.FormatFloat(height, 'f', 0, 64)

	var blockHeightScript []byte

	blockHeightScript = append(blockHeightScript, util.HexToBytes(length)...)
	blockHeightScript = append(blockHeightScript, util.ReverseBuffer(util.HexToBytes(blockHeightSerial))...)
	blockHeightScript = append(blockHeightScript, []byte{0}...)

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

	// Calculate which founder
	index := int(math.Floor(float64(blockHeight) / util.FoundersRewardAddressChangeInterval))

	poolAddressScriptFormat, _ := zaddr.DecodeAddress(poolAddress, &chaincfg.TestNet3Params)
	poolScript, _ := zaddr.PayToAddrScript(poolAddressScriptFormat)
	outputPool := zecl.Output{
		Value:        util.GetConstReward(blockHeight).Int64() + feeReward,
		ScriptPubKey: poolScript,
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
		Outputs:               []zecl.Output{outputPool},
	}

	if blockHeight < util.FirstRewardHalvingBlock {
		foundersAddressScriptFormat, _ := zaddr.DecodeAddress(util.TestFoundersRewardAddresses[index], &chaincfg.TestNet3Params)
		founderScript, _ := zaddr.PayToAddrScript(foundersAddressScriptFormat)
		outputFounders := zecl.Output{
			Value:        foundersRewardZatoshi,
			ScriptPubKey: founderScript,
		}

		transaction.Outputs = append(transaction.Outputs, outputFounders)
	}

	transactionBytes, _ := transaction.MarshalBinary()

	return transactionBytes, transaction.TxHash()
}
