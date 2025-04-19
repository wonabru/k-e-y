package account

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/common/hexutil"
	"log"
	"math"
	"strconv"
)

type Account struct {
	Balance            int64                        `json:"balance"`
	Address            [common.AddressLength]byte   `json:"address"`
	TransactionDelay   int64                        `json:"transactionDelay"`
	MultiSignNumber    uint8                        `json:"multiSignNumber"`
	MultiSignAddresses [][common.AddressLength]byte `json:"multiSignAddresses,omitempty"`
}

func GetAccountByAddressBytes(address []byte) Account {
	AccountsRWMutex.RLock()
	defer AccountsRWMutex.RUnlock()
	addrb := [common.AddressLength]byte{}
	copy(addrb[:], address[:common.AddressLength])
	return Accounts.AllAccounts[addrb]
}

func CanBeModifiedAccount(address []byte) bool {
	acc := GetAccountByAddressBytes(address)
	return acc.MultiSignNumber == 0 && acc.TransactionDelay == 0
}

func (a *Account) ModifyAccountToEscrow(transactionDelay int64) error {
	if a.TransactionDelay > 0 {
		return fmt.Errorf("account is just escrow and cannot be modified")
	}
	if transactionDelay == 0 {
		return fmt.Errorf("transaction delay in escrow must be larger than 0")
	}
	if transactionDelay > common.MaxTransactionDelay {
		return fmt.Errorf("transaction delay in escrow must be less than %v", common.MaxTransactionDelay)
	}
	a.TransactionDelay = transactionDelay
	AccountsRWMutex.Lock()
	Accounts.AllAccounts[a.Address] = *a
	AccountsRWMutex.Unlock()
	return nil
}

func (a *Account) ModifyAccountToMultiSign(numApprovals uint8, addresses []common.Address) error {
	if a.MultiSignNumber > 0 {
		return fmt.Errorf("account is just MultiSign and cannot be modified")
	}
	if int(numApprovals) == 0 {
		return fmt.Errorf("MultiSign must have at least 1 Approval account")
	}
	if int(numApprovals) > len(addresses) {
		return fmt.Errorf("number of MultiSign approval addresses must be larger than number of Approvals %v", numApprovals)
	}
	a.MultiSignNumber = numApprovals

	addrs := make([][common.AddressLength]byte, len(addresses))
	for i, a := range addresses {
		copy(addrs[i][:], a.GetBytes())
	}
	a.MultiSignAddresses = addrs
	AccountsRWMutex.Lock()
	Accounts.AllAccounts[a.Address] = *a
	AccountsRWMutex.Unlock()
	return nil
}

func SetAccountByAddressBytes(address []byte) Account {
	dexAccount := GetAccountByAddressBytes(address)
	if !bytes.Equal(dexAccount.Address[:], address) {
		log.Println("no account found, will be created")
		addrb := [common.AddressLength]byte{}
		copy(addrb[:], address[:common.AddressLength])
		dexAccount = Account{
			Balance:          0,
			Address:          addrb,
			TransactionDelay: 0,
			MultiSignNumber:  0,
		}
		AccountsRWMutex.Lock()
		Accounts.AllAccounts[addrb] = dexAccount
		AccountsRWMutex.Unlock()
	}
	return dexAccount
}

// GetBalanceConfirmedFloat get amount of confirmed QWIDT in human-readable format
func (a *Account) GetBalanceConfirmedFloat() float64 {
	return float64(a.Balance) * math.Pow10(-int(common.Decimals))
}

func (a Account) Marshal() []byte {
	b := common.GetByteInt64(a.Balance)
	b = append(b, a.Address[:]...)
	delay := common.GetByteInt64(a.TransactionDelay)
	b = append(b, delay...)
	b = append(b, a.MultiSignNumber)
	for _, msa := range a.MultiSignAddresses {
		b = append(b, msa[:]...)
	}
	return b
}

func (a *Account) Unmarshal(data []byte) error {
	if len(data) < 37 {
		return fmt.Errorf("wrong number of bytes in unmarshal account")
	}
	a.Balance = common.GetInt64FromByte(data[:8])

	copy(a.Address[:], data[8:28])
	a.TransactionDelay = common.GetInt64FromByte(data[28:36])
	a.MultiSignNumber = data[36]
	if len(data) > 37 {
		data = data[37:]
		lenAccMS := len(data) / 20
		if int(a.MultiSignNumber) > lenAccMS {
			return fmt.Errorf("wrongly defined multisign account")
		}
		if lenAccMS > 0 {
			a.MultiSignAddresses = make([][common.AddressLength]byte, lenAccMS)
			for i := 0; i < lenAccMS; i++ {
				copy(a.MultiSignAddresses[i][:], data[:20])
				data = data[20:]
			}
		}
	}

	return nil
}

func (a Account) GetString() string {
	r := "Address: " + hexutil.Encode(a.Address[:]) + "\n"
	r += "Balance: " + strconv.FormatInt(a.Balance, 10) + "\n"
	if a.TransactionDelay > 0 {
		r += "Escrow account with "
		r += "Transactions Delayed: " + strconv.FormatInt(a.TransactionDelay, 10) + " blocks\n"
	}
	if a.MultiSignNumber > 0 {
		r += "Multi Signature account with \n"
		r += "Signatures: " + strconv.FormatInt(int64(a.MultiSignNumber), 10) + "/" + strconv.FormatInt(int64(len(a.MultiSignAddresses)), 10) + "\n"
		r += "Multi Signature Addresses: \n"
		for i, msa := range a.MultiSignAddresses {
			r += "\t" + strconv.FormatInt(int64(i), 10) + ": " + hexutil.Encode(msa[:]) + "\n"
		}
	}
	return r
}
