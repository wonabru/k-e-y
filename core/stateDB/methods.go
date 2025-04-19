package stateDB

import (
	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/core/types"
	"github.com/okuralabs/okura-node/crypto"
	"math/big"
)

type TokenInfo struct {
	Name     string `json:"name"`
	Symbols  string `json:"symbols"`
	Decimals uint8  `json:"decimals"`
}

type StateAccount struct {
	Accounts            map[[common.AddressLength]byte]account.Account                      `json:"accounts"`
	Codes               map[[common.AddressLength]byte][]byte                               `json:"codes"`
	CodeHashes          map[[common.AddressLength]byte]common.Hash                          `json:"codeHashes"`
	StatesHashes        map[[common.AddressLength]byte]map[common.Hash]common.Hash          `json:"statesHashes"`
	Nonces              map[[common.AddressLength]byte]uint64                               `json:"nonces"`
	States              map[common.Hash][]byte                                              `json:"states"`
	Balances            map[[common.AddressLength]byte]map[[common.AddressLength]byte]int64 `json:"balances"`
	Tokens              map[[common.AddressLength]byte]TokenInfo                            `json:"tokens"`
	SnapShotNum         int                                                                 `json:"snapShotNum"`
	SnapShotPreimage    map[int]map[[common.AddressLength]byte]common.Hash                  `json:"snapShotPreimage"`
	HeightToSnapShotNum map[int64]int                                                       `json:"HeightToSnapShotNum"` // suppose int should be replaced by int64
}

func CreateStateDB() StateAccount {
	sa := StateAccount{}
	sa.Accounts = map[[common.AddressLength]byte]account.Account{}
	sa.Codes = map[[common.AddressLength]byte][]byte{}
	sa.CodeHashes = map[[common.AddressLength]byte]common.Hash{}
	sa.Nonces = map[[common.AddressLength]byte]uint64{}
	sa.StatesHashes = map[[common.AddressLength]byte]map[common.Hash]common.Hash{}
	sa.States = map[common.Hash][]byte{}
	sa.Balances = map[[common.AddressLength]byte]map[[common.AddressLength]byte]int64{}
	sa.Tokens = map[[common.AddressLength]byte]TokenInfo{}
	sa.SnapShotNum = 0
	sa.SnapShotPreimage = map[int]map[[common.AddressLength]byte]common.Hash{}
	sa.HeightToSnapShotNum = map[int64]int{}
	return sa
}

func (sa *StateAccount) SetSnapShotNum(height int64, snapNum int) {
	(*sa).HeightToSnapShotNum[height] = snapNum
}

func (sa *StateAccount) GetSnapShotNum(height int64) (int, bool) {
	sn, ok := sa.HeightToSnapShotNum[height]
	return sn, ok
}

func (sa *StateAccount) CreateAccount(a common.Address) {
	addrb := [common.AddressLength]byte{}
	copy(addrb[:], a.ByteValue[:])
	acc := account.Account{
		Balance:          0,
		Address:          addrb,
		TransactionDelay: 0,
		MultiSignNumber:  0,
	}
	(*sa).Accounts[a.ByteValue] = acc
}

func (sa *StateAccount) GetAllRegisteredTokens() map[[common.AddressLength]byte]TokenInfo {
	return sa.Tokens
}

func (sa *StateAccount) RegisterNewToken(a common.Address, name string, symbol string, decimals uint8) {
	ti := TokenInfo{
		Name:     name,
		Symbols:  symbol,
		Decimals: decimals,
	}
	(*sa).Tokens[a.ByteValue] = ti
}

func (sa *StateAccount) SubBalance(common.Address, *big.Int) {

}
func (sa *StateAccount) AddBalance(common.Address, *big.Int) {

}
func (sa *StateAccount) GetBalance(common.Address) *big.Int {
	return new(big.Int).SetInt64(0)
}

func (sa *StateAccount) GetNonce(a common.Address) uint64 {
	return sa.Nonces[a.ByteValue]
}
func (sa *StateAccount) SetNonce(a common.Address, n uint64) {
	(*sa).Nonces[a.ByteValue] = n
}

func (sa *StateAccount) GetCodeHash(a common.Address) common.Hash {
	return sa.CodeHashes[a.ByteValue]
}

func (sa *StateAccount) GetCode(a common.Address) []byte {
	return sa.Codes[a.ByteValue]
}

func (sa *StateAccount) SetCode(a common.Address, c []byte) {
	(*sa).Codes[a.ByteValue] = c
	(*sa).CodeHashes[a.ByteValue] = crypto.Keccak256Hash(c)
}

func (sa *StateAccount) GetCodeSize(a common.Address) int {
	return len(sa.Codes[a.ByteValue])
}

func (sa *StateAccount) AddRefund(uint64) {

}
func (sa *StateAccount) SubRefund(uint64) {

}
func (sa *StateAccount) GetRefund() uint64 {
	return 0
}

func (sa *StateAccount) GetCommittedState(a common.Address, h common.Hash) common.Hash {
	s, ok := sa.StatesHashes[a.ByteValue]
	if ok {
		return s[h]
	}
	return common.Hash{}
}
func (sa *StateAccount) GetState(a common.Address, h common.Hash) common.Hash {
	s, ok := sa.StatesHashes[a.ByteValue]
	if ok {
		return s[h]
	}
	return common.Hash{}
}
func (sa *StateAccount) SetState(a common.Address, h common.Hash, h2 common.Hash) {
	(*sa).SnapShotNum++

	_, ok := (*sa).StatesHashes[a.ByteValue]
	if ok {
		(*sa).SnapShotPreimage[(*sa).SnapShotNum] = map[[common.AddressLength]byte]common.Hash{a.ByteValue: (*sa).StatesHashes[a.ByteValue][h]}
		(*sa).StatesHashes[a.ByteValue][h] = h2
		return
	}
	(*sa).SnapShotPreimage[(*sa).SnapShotNum] = map[[common.AddressLength]byte]common.Hash{a.ByteValue: common.EmptyHash()}
	(*sa).StatesHashes[a.ByteValue] = map[common.Hash]common.Hash{}
	(*sa).StatesHashes[a.ByteValue][h] = h2
}

func (sa *StateAccount) Suicide(common.Address) bool {
	return false
}
func (sa *StateAccount) HasSuicided(common.Address) bool {
	return false
}

// Exist reports whether the given account exists in state.
// Notably this should also return true for suicided accounts.
func (sa *StateAccount) Exist(a common.Address) bool {
	_, ok := sa.Accounts[a.ByteValue]
	return ok
}

// Empty returns whether the given account is empty. Empty
// is defined according to EIP161 (balance = nonce = code = 0).
func (sa *StateAccount) Empty(a common.Address) bool {
	return sa.Nonces[a.ByteValue] == 0 && len(sa.Codes[a.ByteValue]) == 0
}

func (sa *StateAccount) PrepareAccessList(sender common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {

}
func (sa *StateAccount) AddressInAccessList(addr common.Address) bool {
	return true
}
func (sa *StateAccount) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return true, true
}

// AddAddressToAccessList adds the given address to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (sa *StateAccount) AddAddressToAccessList(addr common.Address) {

}

// AddSlotToAccessList adds the given (address,slot) to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (sa *StateAccount) AddSlotToAccessList(addr common.Address, slot common.Hash) {

}

func (sa *StateAccount) RevertToSnapshot(sn int) {
	for s := sn + 1; s <= sa.SnapShotNum; s++ {
		for a, h := range sa.SnapShotPreimage[s] {
			if sa.SnapShotPreimage[s][a] == common.EmptyHash() {
				delete((*sa).StatesHashes[a], h)
				continue
			}
			(*sa).StatesHashes[a][h] = sa.SnapShotPreimage[s][a]
		}
	}
	(*sa).SnapShotNum = sn
}

func (sa *StateAccount) Snapshot() int {
	return sa.SnapShotNum
}

func (sa *StateAccount) AddLog(*types.Log) {

}
func (sa *StateAccount) AddPreimage(h common.Hash, b []byte) {
	(*sa).States[h] = b
}

func (sa *StateAccount) GetCoinBalance(acc common.Address, coin common.Address) int64 {
	_, ok := sa.Balances[acc.ByteValue]
	if ok {
		return sa.Balances[acc.ByteValue][coin.ByteValue]
	} else {
		return 0
	}
}

func (sa *StateAccount) SetCoinBalance(acc common.Address, coin common.Address, value int64) {
	_, ok := sa.Balances[acc.ByteValue]
	if ok {
		(*sa).Balances[acc.ByteValue][coin.ByteValue] = value
	} else {
		(*sa).Balances[acc.ByteValue] = map[[common.AddressLength]byte]int64{coin.ByteValue: value}
	}
}

//func (sa *StateAccount) getStateObject(a common.Address) *stateObject {
//
//}

func (sa *StateAccount) ForEachStorage(a common.Address, cb func(key common.Hash, value common.Hash) bool) error {

	shs, ok := sa.StatesHashes[a.ByteValue]
	if !ok {
		return nil
	}
	for h, _ := range shs {
		if value, dirty := shs[h]; dirty {
			if !cb(h, value) {
				return nil
			}
			continue
		}
	}

	return nil
}
