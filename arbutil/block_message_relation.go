// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE

package arbutil

// messages are 0-indexed
type MessageIndex uint64

func BlockNumberToMessageCount(blockNumber uint64, genesisBlockNumber uint64) MessageIndex {
	return MessageIndex(blockNumber + 1 - genesisBlockNumber)
}

func MessageCountToBlockNumber(messageCount MessageIndex, genesisBlockNumber uint64) int64 {
	return int64(uint64(messageCount)+genesisBlockNumber) - 1
}
