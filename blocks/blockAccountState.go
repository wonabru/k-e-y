package blocks

import (
	"fmt"
	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/common"
)

func AddBalance(address [common.AddressLength]byte, addedAmount int64) error {
	balance := int64(0)
	account.AccountsRWMutex.Lock()

	if _, ok := account.Accounts.AllAccounts[address]; ok {
		balance = account.Accounts.AllAccounts[address].Balance
	} else {
		acc := account.Account{}
		acc.Balance = balance
		acc.Address = address
		account.Accounts.AllAccounts[address] = acc
	}
	if balance+addedAmount < 0 {
		account.AccountsRWMutex.Unlock()
		return fmt.Errorf("Not enough funds on account")
	}
	balance += addedAmount
	account.AccountsRWMutex.Unlock()
	account.SetBalance(address, balance)
	return nil
}

func GetSupplyInAccounts() int64 {
	sum := int64(0)
	account.AccountsRWMutex.RLock()
	defer account.AccountsRWMutex.RUnlock()
	for _, acc := range account.Accounts.AllAccounts {
		sum += acc.Balance
	}
	return sum
}

func GetSupplyInStakedAccounts() (int64, int64) {
	sumStaked := int64(0)
	sumRewards := int64(0)
	account.StakingRWMutex.RLock()
	defer account.StakingRWMutex.RUnlock()

	for _, delAcc := range account.StakingAccounts {
		for _, acc := range delAcc.AllStakingAccounts {
			sumStaked += acc.StakedBalance
			sumRewards += acc.StakingRewards
		}
	}
	return sumStaked, sumRewards
}
