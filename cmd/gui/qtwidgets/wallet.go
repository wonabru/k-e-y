package qtwidgets

import (
	"fmt"
	"github.com/okuralabs/okura-node/common"
	clientrpc "github.com/okuralabs/okura-node/rpc/client"
	"github.com/okuralabs/okura-node/wallet"
	"github.com/therecipe/qt/widgets"
	"strconv"
)

var err error

func isRegisteredPubKeyInBlockchain() {
	clientrpc.InRPC <- SignMessage([]byte("CHCK"))
	var reply []byte
	reply = <-clientrpc.OutRPC
	if len(reply) > 0 {
		info := string(reply)
		widgets.QMessageBox_Information(nil, "Warning", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
	}
}

func ShowWalletPage() *widgets.QTabWidget {
	// create a regular widget
	// give it a QVBoxLayout
	// and make it the central widget of the window
	widget := widgets.NewQTabWidget(nil)
	widget.SetLayout(widgets.NewQVBoxLayout())

	numberWallet := widgets.NewQLineEdit(nil)
	numberWallet.SetPlaceholderText("Select wallet number (default is 0):")
	widget.Layout().AddWidget(numberWallet)
	// create a line edit
	// with a custom placeholder text
	// and add it to the central widgets layout
	input := widgets.NewQLineEdit(nil)
	input.SetEchoMode(widgets.QLineEdit__Password)
	input.SetPlaceholderText("Password:")
	widget.Layout().AddWidget(input)

	// connect the clicked signal
	// and add it to the central widgets layout
	button := widgets.NewQPushButton2("Load wallet", nil)
	button.ConnectClicked(func(bool) {
		MainWallet = nil
		var info string
		nw := numberWallet.Text()
		if nw == "" {
			nw = "0"
		}
		numWallet, err := strconv.Atoi(nw)
		if err != nil {
			info = fmt.Sprintf("%v", err)
			widgets.QMessageBox_Information(nil, "error", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		if numWallet < 0 || numWallet > 255 {
			info = fmt.Sprintf("wallet number should be less than 255 and more than or equal 0")
			widgets.QMessageBox_Information(nil, "error", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		err = SetCurrentEncryptions()
		if err != nil {
			widgets.QMessageBox_Information(nil, "Error", "error with retrieving current encryption", widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		MainWallet, err = wallet.Load(uint8(numWallet), input.Text())
		if err != nil {
			info = fmt.Sprintf("%v", err)
			widgets.QMessageBox_Information(nil, "error", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}

		if MainWallet.GetSigName(true) != common.SigName() {
			widgets.QMessageBox_Information(nil, "Warning", "primary encryption has changed. You need to update wallet", widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
		}
		if MainWallet.GetSigName(false) != common.SigName2() {
			widgets.QMessageBox_Information(nil, "Warning", "secondary encryption has changed. You need to update wallet", widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
		}
		if err != nil {
			info = fmt.Sprintf("%v", err)
		} else if MainWallet.Check() {
			info = MainWallet.ShowInfo()
		} else {
			info = fmt.Sprintf("no wallet exists with this number %v", numWallet)
		}

		widgets.QMessageBox_Information(nil, "OK", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)

		isRegisteredPubKeyInBlockchain()

	})
	widget.Layout().AddWidget(button)

	buttonAddWallet := widgets.NewQPushButton2("Update wallet with new encryption", nil)
	buttonAddWallet.ConnectReleased(func() {
		MainWallet = nil
		info := "Updating the wallet was successful"
		err = SetCurrentEncryptions()
		if err != nil {
			widgets.QMessageBox_Information(nil, "Error", "error with retrieving current encryption", widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		nw := numberWallet.Text()
		if nw == "" {
			nw = "0"
		}
		numWallet, err := strconv.Atoi(nw)
		if err != nil {
			info = fmt.Sprintf("%v", err)
			widgets.QMessageBox_Information(nil, "OK", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		MainWallet, err = wallet.Load(uint8(numWallet), input.Text())

		if MainWallet != nil && MainWallet.Check() || err != nil {

			if MainWallet.GetSigName(true) != common.SigName() {

				err := MainWallet.AddNewEncryptionToActiveWallet(common.SigName(), true)
				if err != nil {
					widgets.QMessageBox_Information(nil, "Error", err.Error(), widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
					return
				}
			}
			if MainWallet.GetSigName(false) != common.SigName2() {
				err := MainWallet.AddNewEncryptionToActiveWallet(common.SigName2(), false)
				if err != nil {
					widgets.QMessageBox_Information(nil, "Error", err.Error(), widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
					return
				}
			}

		}

		err = MainWallet.Store(true)
		if err != nil {
			info = fmt.Sprintf("%v", err)
			widgets.QMessageBox_Information(nil, "OK", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}

		if MainWallet.Check() {
			info = MainWallet.ShowInfo()
		}

		widgets.QMessageBox_Information(nil, "OK", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
		//buttonNewWallet.SetDisabled(true)
	})

	widget.Layout().AddWidget(buttonAddWallet)

	buttonNewWallet := widgets.NewQPushButton2("Generate new wallet", nil)
	buttonNewWallet.ConnectReleased(func() {
		MainWallet = nil
		info := "Creating reserve wallet success"

		nw := numberWallet.Text()
		if nw == "" {
			nw = "0"
		}
		numWallet, err := strconv.Atoi(nw)
		if err != nil {
			info = fmt.Sprintf("%v", err)
			widgets.QMessageBox_Information(nil, "OK", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		err = SetCurrentEncryptions()
		if err != nil {
			widgets.QMessageBox_Information(nil, "Error", "error with retrieving current encryption", widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		MainWallet, err = wallet.Load(uint8(numWallet), input.Text())
		if MainWallet != nil && MainWallet.Check() || err != nil {
			info = fmt.Sprintf("Wallet number %v exists!!! Would you like to overwrite? Current wallet will be removed permanently if overwritten.", numWallet)
			overwrite := widgets.QMessageBox_Question(nil, "Would you like to overwrite?", info, widgets.QMessageBox__No|widgets.QMessageBox__Yes, widgets.QMessageBox__No)
			if overwrite == widgets.QMessageBox__No {
				return
			}

		}
		MainWallet, err = wallet.GenerateNewWallet(uint8(numWallet), input.Text())
		err = StoreWalletNewGenerated(MainWallet)
		if err != nil {
			info = fmt.Sprintf("%v", err)
			widgets.QMessageBox_Information(nil, "OK", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}

		if err != nil {
			info = fmt.Sprintf("Can not store wallet. Error %v", err)
		} else if MainWallet.Check() {
			info = MainWallet.ShowInfo()
		} else {
			info = fmt.Sprintf("no wallet exists with this number %v", numWallet)
		}

		widgets.QMessageBox_Information(nil, "OK", info, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
		//buttonNewWallet.SetDisabled(true)
	})

	widget.Layout().AddWidget(buttonNewWallet)
	newPassword := widgets.NewQLineEdit(nil)
	newPassword.SetEchoMode(widgets.QLineEdit__Password)
	newPassword.SetPlaceholderText("New password:")
	widget.Layout().AddWidget(newPassword)
	repeatPassword := widgets.NewQLineEdit(nil)
	repeatPassword.SetEchoMode(widgets.QLineEdit__Password)
	repeatPassword.SetPlaceholderText("Repeat password:")
	widget.Layout().AddWidget(repeatPassword)
	buttonChangePassword := widgets.NewQPushButton2("Change password", nil)
	buttonChangePassword.ConnectClicked(func(bool) {
		if MainWallet.GetSecretKey().GetLength() == 0 {
			widgets.QMessageBox_Information(nil, "Error", "Load wallet first", widgets.QMessageBox__Close, widgets.QMessageBox__Close)
			return
		}
		if newPassword.Text() != repeatPassword.Text() {

			widgets.QMessageBox_Information(nil, "Error", "Passwords do not match", widgets.QMessageBox__Close, widgets.QMessageBox__Close)
			return
		}
		MainWallet.SetPassword(newPassword.Text())
		err = MainWallet.Store(false)
		if err != nil {
			widgets.QMessageBox_Information(nil, "Error", fmt.Sprintf("%v", err), widgets.QMessageBox__Close, widgets.QMessageBox__Close)
			return
		}
		widgets.QMessageBox_Information(nil, "OK", "Password changed", widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
	})
	widget.Layout().AddWidget(buttonChangePassword)
	buttonMnemonic := widgets.NewQPushButton2("Show mnemonic words", nil)
	buttonMnemonic.ConnectClicked(func(bool) {
		var v string
		mnemonic, err := MainWallet.GetMnemonicWords(true)
		if err != nil {
			v = err.Error()
		} else {
			v = fmt.Sprintf("Mnemonic words for primary encryption:\n%v", mnemonic)
		}
		mnemonic2, err := MainWallet.GetMnemonicWords(false)
		if err != nil {
			v += err.Error()
		} else {
			v += fmt.Sprintf("\nMnemonic words for secondary encryption:\n%v", mnemonic2)
		}
		widgets.QMessageBox_Information(nil, "OK", v, widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
	})
	widget.Layout().AddWidget(buttonMnemonic)

	inputRestoreMnemonic := widgets.NewQLineEdit(nil)
	inputRestoreMnemonic.SetPlaceholderText("Mnemonic words seperated by space:")
	widget.Layout().AddWidget(inputRestoreMnemonic)
	buttonRestoreMnemonic := widgets.NewQPushButton2("Restore private key from mnemonic words", nil)
	buttonRestoreMnemonic.ConnectClicked(func(bool) {
		err := MainWallet.RestoreSecretKeyFromMnemonic(inputRestoreMnemonic.Text(), true)
		if err != nil {
			widgets.QMessageBox_Information(nil, "OK", fmt.Sprintf("Can not restore primary Private key from mnemonic words:\n%v", err), widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)

		} else {
			sec := MainWallet.GetSecretKey()
			widgets.QMessageBox_Information(nil, "OK", fmt.Sprintf("Primary Private Key:\n%v", sec.GetHex()), widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		err = MainWallet.RestoreSecretKeyFromMnemonic(inputRestoreMnemonic.Text(), false)
		if err != nil {
			widgets.QMessageBox_Information(nil, "OK", fmt.Sprintf("Can not restore secondary Private key from mnemonic words:\n%v", err), widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)
			return
		}
		sec := MainWallet.GetSecretKey2()
		widgets.QMessageBox_Information(nil, "OK", fmt.Sprintf("Secondary Private Key:\n%v", sec.GetHex()), widgets.QMessageBox__Ok, widgets.QMessageBox__Ok)

	})
	widget.Layout().AddWidget(buttonRestoreMnemonic)
	return widget
}
