package stateDB

import "github.com/okuralabs/okura-node/common"

type Code []byte

type Storage map[common.Hash]common.Hash

type stateObject struct {
	address common.Address
	db      *StateAccount
	code    Code

	originStorage Storage
}
