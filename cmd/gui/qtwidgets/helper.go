package qtwidgets

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/blocks"
	"github.com/okuralabs/okura-node/common"
	clientrpc "github.com/okuralabs/okura-node/rpc/client"
	"github.com/okuralabs/okura-node/wallet"
	"log"
	"os"
)

func SignMessage(line []byte) []byte {

	operation := string(line[0:4])
	verificationNeeded := true
	for _, noVerification := range common.ConnectionsWithoutVerification {
		if bytes.Equal([]byte(operation), noVerification) {
			verificationNeeded = false
			break
		}
	}
	if verificationNeeded {
		if MainWallet == nil || (!MainWallet.Check() || !MainWallet.Check2()) {
			log.Println("wallet not loaded yet")
			return line
		}
		if common.IsPaused() == false {
			// primary encryption used
			line = common.BytesToLenAndBytes(line)
			sign, err := MainWallet.Sign(line, true)
			if err != nil {
				log.Println(err)
				return line
			}
			line = append(line, sign.GetBytes()...)

		} else {
			// secondary encryption
			line = common.BytesToLenAndBytes(line)
			sign, err := MainWallet.Sign(line, false)
			if err != nil {
				log.Println(err)
				return line
			}
			line = append(line, sign.GetBytes()...)
		}
	} else {
		line = common.BytesToLenAndBytes(line)
	}
	return line
}

func StoreWalletNewGenerated(w *wallet.Wallet) error {
	if err != nil {
		log.Printf("Can not create wallet. Error %v", err)
	}
	folderPath := w.HomePath
	err = os.MkdirAll(w.HomePath, 0755)
	if err != nil {
		log.Fatal(err)
	}
	fileInfo, err := os.Stat(folderPath)
	if err != nil {
		return fmt.Errorf("Error getting folder info: %v", err)
	}
	// Get the folder permissions
	permissions := fileInfo.Mode().Perm()
	fmt.Printf("Folder permissions: %v\n", permissions)
	// Check if the current user has read, write, and execute permissions
	hasReadPermission := permissions&0400 != 0
	hasWritePermission := permissions&0200 != 0
	hasExecutePermission := permissions&0100 != 0
	fmt.Printf("Read permission: %v\n", hasReadPermission)
	fmt.Printf("Write permission: %v\n", hasWritePermission)
	fmt.Printf("Execute permission: %v\n", hasExecutePermission)

	folderPath2 := w.HomePath2
	err = os.MkdirAll(w.HomePath2, 0755)
	if err != nil {
		return err
	}
	fileInfo2, err := os.Stat(folderPath2)
	if err != nil {
		return fmt.Errorf("Error getting folder info: %v", err)
	}
	// Get the folder permissions
	permissions2 := fileInfo2.Mode().Perm()
	fmt.Printf("Folder permissions: %v\n", permissions2)
	// Check if the current user has read, write, and execute permissions
	hasReadPermission2 := permissions2&0400 != 0
	hasWritePermission2 := permissions2&0200 != 0
	hasExecutePermission2 := permissions2&0100 != 0
	fmt.Printf("Read permission: %v\n", hasReadPermission2)
	fmt.Printf("Write permission: %v\n", hasWritePermission2)
	fmt.Printf("Execute permission: %v\n", hasExecutePermission2)

	err = w.Store(true)
	if err != nil {
		return err
	}
	return nil
}

func SetCurrentEncryptions() error {
	clientrpc.InRPC <- SignMessage([]byte("ENCR"))
	var reply []byte
	reply = <-clientrpc.OutRPC
	if bytes.Equal(reply, []byte("Timeout")) {
		return fmt.Errorf("timout")
	}
	enc1b, left, err := common.BytesWithLenToBytes(reply)
	if err != nil {
		return err
	}
	enc2b, left, err := common.BytesWithLenToBytes(left)
	if err != nil {
		return err
	}
	enc1, err := blocks.FromBytesToEncryptionConfig(enc1b, true)
	if err != nil {
		return err
	}
	common.SetEncryption(enc1.SigName, enc1.PubKeyLength, enc1.PrivateKeyLength, enc1.SignatureLength, enc1.IsPaused, true)
	enc2, err := blocks.FromBytesToEncryptionConfig(enc2b, false)
	if err != nil {
		return err
	}
	common.SetEncryption(enc2.SigName, enc2.PubKeyLength, enc2.PrivateKeyLength, enc2.SignatureLength, enc2.IsPaused, false)
	return nil
}
