package qtwidgets

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/common"
	clientrpc "github.com/okuralabs/okura-node/rpc/client"
	"github.com/okuralabs/okura-node/statistics"
	"github.com/okuralabs/okura-node/wallet"
	"github.com/therecipe/qt/widgets"
	"strconv"
	"strings"

	"log"
)

var StatsLabel *widgets.QLabel
var MainWallet *wallet.Wallet

func UpdateAccountStats() {
	if (MainWallet == nil) || ((MainWallet != nil) && MainWallet.Check() == false) {
		return
	}
	clientrpc.InRPC <- SignMessage([]byte("STAT"))
	var reply []byte
	reply = <-clientrpc.OutRPC
	if bytes.Equal(reply, []byte("Timeout")) {
		return
	}
	sm := statistics.GetStatsManager()
	st := sm.Stats
	err := common.Unmarshal(reply, common.StatDBPrefix, &st)
	if err != nil {
		log.Println("Can not unmarshal statistics")
		common.SetIsPaused(!common.IsPaused(), true)
		return
	}
	if st.Height == 0 {
		return
	}
	txt := fmt.Sprintln("Height:", st.Height)
	txt += fmt.Sprintln("Heights max:", st.HeightMax)
	txt += fmt.Sprintln("Time interval [sec.]:", st.TimeInterval)
	txt += fmt.Sprintln("Difficulty:", st.Difficulty)
	txt += fmt.Sprintln("\nPrice Oracle:", st.PriceOracle, " QWIDT/USD")
	txt += fmt.Sprintln("Rand Oracle:", st.RandOracle)
	txt += fmt.Sprintln("\nNumber of transactions : ", st.Transactions, "/", st.TransactionsPending)
	txt += fmt.Sprintln("Size of transactions [kB] : ", st.TransactionsSize/1024, "/", st.TransactionsPendingSize/1024)
	txt += fmt.Sprintln("TPS:", st.Tps)
	if st.Syncing {
		txt += fmt.Sprintln("Syncing...")
	}
	if MainWallet.Check() == false {
		return
	}
	inb := append([]byte("ACCT"), MainWallet.Address.GetBytes()...)
	clientrpc.InRPC <- SignMessage(inb)
	var re []byte
	var acc account.Account

	re = <-clientrpc.OutRPC
	if bytes.Equal(reply, []byte("Timeout")) {
		return
	}
	err = acc.Unmarshal(re)
	if err != nil {
		log.Println("cannot unmarshal account")
		common.SetIsPaused(!common.IsPaused(), true)
		return
	}
	conf := acc.GetBalanceConfirmedFloat()
	uncTx := 0.0

	stake := 0.0
	uncStake := 0.0

	rewards := 0.0
	uncRewards := 0.0
	locks := 0.0
	var stakeAccs [256]account.StakingAccount
	for i := 1; i < 5; i++ { // should be 256
		if MainWallet.Check() == false {
			return
		}
		inb = append([]byte("STAK"), MainWallet.Address.GetBytes()...)
		inb = append(inb, byte(i))
		clientrpc.InRPC <- SignMessage(inb)
		re = <-clientrpc.OutRPC
		if string(reply) == "Timeout" {
			return
		}
		err = stakeAccs[i].Unmarshal(re[:len(re)-8])
		if err != nil {
			log.Println("cannot unmarshal stake account")
			common.SetIsPaused(!common.IsPaused(), true)
			return
		}
		stake += account.Int64toFloat64(stakeAccs[i].StakedBalance)
		rewards += account.Int64toFloat64(stakeAccs[i].StakingRewards)
		locks += account.Int64toFloat64(common.GetInt64FromByte(re[len(re)-8:]))
	}

	txt += fmt.Sprintln("\n\nYour Address:", MainWallet.Address.GetHex())
	txt += fmt.Sprintf("Your holdings: %18.8f QWIDT\n", conf+stake+rewards+uncTx+uncStake+uncRewards)
	txt += fmt.Sprintf("Confirmed balance: %18.8f QWIDT\n", conf)
	//txt += fmt.Sprintf("Transactions unconfirmed balance: %18.8f QWIDT\n", uncTx)
	txt += fmt.Sprintf("Staked amount: %18.8f QWIDT\n", stake)
	txt += fmt.Sprintf("Locked amount: %18.8f QWIDT\n", locks)
	//txt += fmt.Sprintf("Unconfirmed staked amount: %18.8f QWIDT\n", uncStake)
	txt += fmt.Sprintf("Rewards amount: %18.8f QWIDT\n", rewards)
	//txt += fmt.Sprintf("Unconfirmed rewards amount: %18.8f QWIDT\n", uncRewards)
	txt += fmt.Sprintf("\nStaking details:\n")
	for i, acc := range stakeAccs {
		if acc.StakedBalance == 0 && acc.StakingRewards == 0 {
			continue
		}
		a := common.Address{}
		a.Init(acc.DelegatedAccount[:])

		txt += fmt.Sprintf("Delegated Address: %v\n", a.GetHex())
		txt += fmt.Sprintf("Staked: %v = %v\n", i, account.Int64toFloat64(acc.StakedBalance))
		txt += fmt.Sprintf("Rewards: %v = %v\n", i, account.Int64toFloat64(acc.StakingRewards))
	}
	StatsLabel.SetText(txt)
	//txt2 := ""
	//if lastSt.Heights == 0 {
	//	lastSt = st
	//	lastSt.Heights = 1
	//}
	//histState := acc.GetHistoryState(lastSt.Heights, st.Heights)
	//histRewards := acc.GetHistoryUnconfirmedRewards(lastSt.Heights, st.Heights)
	//histConfirmed := acc.GetHistoryConfirmedTransaction(lastSt.Heights, st.Heights)
	//
	//histStake := acc.GetHistoryStake(lastSt.Heights, st.Heights)
	//for i := st.Heights - 1; i >= lastSt.Heights; i-- {
	//	txt2 += fmt.Sprintln(i, "/", st.Heights, ":")
	//	txt2 += fmt.Sprintln("Balance:", account.Int64toFloat64(histState[i]))
	//	txt2 += fmt.Sprintln("Staked:", account.Int64toFloat64(histStake[i]))
	//	txt2 += fmt.Sprintln("Unconfirmed reward:", account.Int64toFloat64(histRewards[i]))
	//
	//	for k, v := range histConfirmed[i] {
	//		if v != 0 {
	//			txt2 += fmt.Sprintln("Confirmed", k, account.Int64toFloat64(v))
	//		}
	//	}
	//}
	AddNewHistoryItem(txt)
	//lastSt = st
}

func ShowAccountPage() *widgets.QTabWidget {
	// create a regular widget
	// give it a QVBoxLayout
	// and make it the central widget of the window
	widget := widgets.NewQTabWidget(nil)
	widget.SetLayout(widgets.NewQVBoxLayout())

	ipLineEdit := widgets.NewQLineEdit(nil)
	ipLineEdit.SetText("192.168.0.139")
	widget.Layout().AddWidget(ipLineEdit)

	miningCheckBox := widgets.NewQCheckBox(nil)
	miningCheckBox.SetText("Start mining")
	widget.Layout().AddWidget(miningCheckBox)
	miningCheckBox.ConnectClicked(func(bool) {
		miningCheckBox.SetEnabled(false)
		var info *string
		v := "No reply"
		info = &v
		defer func(nfo *string) {
			widgets.QMessageBox_Information(nil, "Info", *nfo, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
		}(info)
		ips := strings.Split(ipLineEdit.Text(), ".")
		if len(ips) != 4 {
			v = "Invalid IP address format"
			return
		}
		var ip [4]byte
		for i := 0; i < 4; i++ {
			num, err := strconv.Atoi(ips[i])
			if err != nil {
				v = fmt.Sprintf("Invalid IP address segment:", ips[i])
				return
			}
			ip[i] = byte(num)
		}
		v = startMining(ip)
		return
	})

	// create a line edit
	// with a custom placeholder text
	// and add it to the central widgets layout
	StatsLabel = widgets.NewQLabel2("Your holdings:", nil, widget.WindowType())
	widget.Layout().AddWidget(StatsLabel)

	return widget
}

func startMining(ip [4]byte) string {
	clientrpc.InRPC <- SignMessage(append([]byte("MINE"), ip[:]...))
	var reply []byte
	reply = <-clientrpc.OutRPC
	if string(reply) == "Timeout" {
		return "Timeout"
	}
	if len(reply) > 0 {
		return string(reply)
	}
	return "No reply"
}
