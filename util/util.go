package util

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"
	"regexp"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
)

var Ether = math.BigPow(10, 18)
var Shannon = math.BigPow(10, 9)
var PowLimitMain = new(big.Int).Sub(math.BigPow(2, 243), big.NewInt(1))
var PowLimitTest = new(big.Int).Sub(math.BigPow(2, 251), big.NewInt(1))

var pow256 = math.BigPow(2, 256)
var addressPattern = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
var tAddressPattern = regexp.MustCompile("^t[0-9a-zA-Z]{34}$")
var loginPattern = regexp.MustCompile("^[[:alnum:]]{,40}$")
var zeroHash = regexp.MustCompile("^0?x?0+$")

var FoundersRewardAddressChangeInterval = 17709.3125
var TestFoundersRewardAddresses = [48]string{
	"t2UNzUUx8mWBCRYPRezvA363EYXyEpHokyi",
	"t2N9PH9Wk9xjqYg9iin1Ua3aekJqfAtE543",
	"t2NGQjYMQhFndDHguvUw4wZdNdsssA6K7x2",
	"t2ENg7hHVqqs9JwU5cgjvSbxnT2a9USNfhy",
	"t2BkYdVCHzvTJJUTx4yZB8qeegD8QsPx8bo",
	"t2J8q1xH1EuigJ52MfExyyjYtN3VgvshKDf",
	"t2Crq9mydTm37kZokC68HzT6yez3t2FBnFj",
	"t2EaMPUiQ1kthqcP5UEkF42CAFKJqXCkXC9",
	"t2F9dtQc63JDDyrhnfpzvVYTJcr57MkqA12",
	"t2LPirmnfYSZc481GgZBa6xUGcoovfytBnC",
	"t26xfxoSw2UV9Pe5o3C8V4YybQD4SESfxtp",
	"t2D3k4fNdErd66YxtvXEdft9xuLoKD7CcVo",
	"t2DWYBkxKNivdmsMiivNJzutaQGqmoRjRnL",
	"t2C3kFF9iQRxfc4B9zgbWo4dQLLqzqjpuGQ",
	"t2MnT5tzu9HSKcppRyUNwoTp8MUueuSGNaB",
	"t2AREsWdoW1F8EQYsScsjkgqobmgrkKeUkK",
	"t2Vf4wKcJ3ZFtLj4jezUUKkwYR92BLHn5UT",
	"t2K3fdViH6R5tRuXLphKyoYXyZhyWGghDNY",
	"t2VEn3KiKyHSGyzd3nDw6ESWtaCQHwuv9WC",
	"t2F8XouqdNMq6zzEvxQXHV1TjwZRHwRg8gC",
	"t2BS7Mrbaef3fA4xrmkvDisFVXVrRBnZ6Qj",
	"t2FuSwoLCdBVPwdZuYoHrEzxAb9qy4qjbnL",
	"t2SX3U8NtrT6gz5Db1AtQCSGjrpptr8JC6h",
	"t2V51gZNSoJ5kRL74bf9YTtbZuv8Fcqx2FH",
	"t2FyTsLjjdm4jeVwir4xzj7FAkUidbr1b4R",
	"t2EYbGLekmpqHyn8UBF6kqpahrYm7D6N1Le",
	"t2NQTrStZHtJECNFT3dUBLYA9AErxPCmkka",
	"t2GSWZZJzoesYxfPTWXkFn5UaxjiYxGBU2a",
	"t2RpffkzyLRevGM3w9aWdqMX6bd8uuAK3vn",
	"t2JzjoQqnuXtTGSN7k7yk5keURBGvYofh1d",
	"t2AEefc72ieTnsXKmgK2bZNckiwvZe3oPNL",
	"t2NNs3ZGZFsNj2wvmVd8BSwSfvETgiLrD8J",
	"t2ECCQPVcxUCSSQopdNquguEPE14HsVfcUn",
	"t2JabDUkG8TaqVKYfqDJ3rqkVdHKp6hwXvG",
	"t2FGzW5Zdc8Cy98ZKmRygsVGi6oKcmYir9n",
	"t2DUD8a21FtEFn42oVLp5NGbogY13uyjy9t",
	"t2UjVSd3zheHPgAkuX8WQW2CiC9xHQ8EvWp",
	"t2TBUAhELyHUn8i6SXYsXz5Lmy7kDzA1uT5",
	"t2Tz3uCyhP6eizUWDc3bGH7XUC9GQsEyQNc",
	"t2NysJSZtLwMLWEJ6MH3BsxRh6h27mNcsSy",
	"t2KXJVVyyrjVxxSeazbY9ksGyft4qsXUNm9",
	"t2J9YYtH31cveiLZzjaE4AcuwVho6qjTNzp",
	"t2QgvW4sP9zaGpPMH1GRzy7cpydmuRfB4AZ",
	"t2NDTJP9MosKpyFPHJmfjc5pGCvAU58XGa4",
	"t29pHDBWq7qN4EjwSEHg8wEqYe9pkmVrtRP",
	"t2Ez9KM8VJLuArcxuEkNRAkhNvidKkzXcjJ",
	"t2D5y7J5fpXajLbGrMBQkFg2mFN8fo3n8cX",
	"t2UV2wr1PTaUiybpkV3FdSdGxUJeZdZztyt",
}

func IsValidHexAddress(s string) bool {
	if IsZeroHash(s) || !addressPattern.MatchString(s) {
		return false
	}
	return true
}

func IsValidtAddress(s string) bool {
	return tAddressPattern.MatchString(s)
}

func IsValidLogin(s string) bool {
	return loginPattern.MatchString(s)
}

func IsZeroHash(s string) bool {
	return zeroHash.MatchString(s)
}

func MakeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func GetTargetHex(diff int64) string {
	var result [32]uint8
	difficulty := big.NewInt(diff)
	bytes := new(big.Int).Div(PowLimitTest, difficulty).Bytes()
	copy(result[len(result)-len(bytes):], bytes)

	return BytesToHex(result[:])
}

func TargetHexToDiff(targetHex string) *big.Int {
	targetBytes := common.FromHex(targetHex)
	return new(big.Int).Div(pow256, new(big.Int).SetBytes(targetBytes))
}

func ToHex(n int64) string {
	return "0x0" + strconv.FormatInt(n, 16)
}

func FormatReward(reward *big.Int) string {
	return reward.String()
}

func FormatRatReward(reward *big.Rat) string {
	wei := new(big.Rat).SetInt(Ether)
	reward = reward.Quo(reward, wei)
	return reward.FloatString(8)
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func MustParseDuration(s string) time.Duration {
	value, err := time.ParseDuration(s)
	if err != nil {
		panic("util: Can't parse duration `" + s + "`: " + err.Error())
	}
	return value
}

func String2Big(num string) *big.Int {
	n := new(big.Int)
	n.SetString(num, 0)
	return n
}

func ReverseBuffer(buffer []byte) []byte {
	for i, j := 0, len(buffer)-1; i < j; i, j = i+1, j-1 {
		buffer[i], buffer[j] = buffer[j], buffer[i]
	}
	return buffer
}

func HexToBytes(hexString string) []byte {
	result, _ := hex.DecodeString(hexString)
	return result
}

func BytesToHex(bytes []byte) string {
	return hex.EncodeToString(bytes)
}

func PackUInt16LE(num uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, num)
	return b
}

func PackUInt32LE(num uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, num)
	return b
}

func PackUInt64LE(num uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, num)
	return b
}

func PackUInt16BE(num uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, num)
	return b
}

func PackUInt32BE(num uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, num)
	return b
}

func PackUInt64BE(num uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, num)
	return b
}

func ReverseUInt32(x uint32) uint32 {
	return (uint32(x)&0xff000000)>>24 |
		(uint32(x)&0x00ff0000)>>8 |
		(uint32(x)&0x0000ff00)<<8 |
		(uint32(x)&0x000000ff)<<24
}

func readHex(s string, n int) ([]byte, error) {
	if len(s) > 2*n {
		return nil, errors.New("value oversized")
	}

	bytes, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	if len(bytes) != n {
		// Pad with zeros
		buf := make([]byte, n)
		copy(buf[n-len(bytes):], bytes)
		buf = bytes
	}

	return bytes, nil
}

func HexToUInt32(s string) uint32 {
	data, err := readHex(s, 4)
	if err != nil {
		return 0
	}

	return binary.BigEndian.Uint32(data)
}

func Sha256d(decrypted []byte) [32]byte {
	round1 := sha256.Sum256(decrypted)
	return sha256.Sum256(round1[:])
}

// func HexToInt

// func IntToByte

// func ByteToInt
