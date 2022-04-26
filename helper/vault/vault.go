package vault

import (
	"math/big"

	"github.com/dogechain-lab/jury/chain"
	"github.com/dogechain-lab/jury/helper/hex"
	"github.com/dogechain-lab/jury/types"
)

// getStorageIndexes is a helper function for getting the correct indexes
// of the storage slots which need to be modified during bootstrap.
//
// It is SC dependant, and based on the SC located at:
// https://github.com/dogechain-lab/jury-contracts
func getStorageIndexes() *StorageIndexes {
	storageIndexes := StorageIndexes{}

	// Get the indexes for _owner, _minimumThreshold
	// Index for regular types is calculated as just the regular slot
	storageIndexes.OwnerIndex = big.NewInt(ownerSlot).Bytes()

	return &storageIndexes
}

// PredeployParams contains the values used to predeploy the Vault contract
type PredeployParams struct {
	Owner types.Address
}

// StorageIndexes is a wrapper for different storage indexes that
// need to be modified
type StorageIndexes struct {
	OwnerIndex []byte // address
}

// Slot definitions for SC storage
const (
	ownerSlot = int64(iota) // Slot 0
)

const (
	//nolint: lll
	VaultSCBytecode = "0x60806040526004361061004e5760003560e01c8063715018a6146100a85780638da5cb5b146100bf5780639a99b4f0146100ea578063b69ef8a814610113578063f2fde38b1461013e576100a3565b366100a35760003411156100a157343373ffffffffffffffffffffffffffffffffffffffff167f053ff9ee923a2e532f5c526d902fb98b6e28e4175a17419e1e4ef1ce110ed43c60405160405180910390a35b005b600080fd5b3480156100b457600080fd5b506100bd610167565b005b3480156100cb57600080fd5b506100d4610201565b6040516100e1919061064b565b60405180910390f35b3480156100f657600080fd5b50610111600480360381019061010c91906105a7565b61022a565b005b34801561011f57600080fd5b50610128610365565b60405161013591906106a6565b60405180910390f35b34801561014a57600080fd5b506101656004803603810190610160919061057a565b61036d565b005b3373ffffffffffffffffffffffffffffffffffffffff1660008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16146101f5576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016101ec90610686565b60405180910390fd5b6101ff6000610477565b565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905090565b3373ffffffffffffffffffffffffffffffffffffffff1660008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16146102b8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016102af90610686565b60405180910390fd5b60004782106102c757476102c9565b815b90506000811115610360578273ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f1935050505015801561031a573d6000803e3d6000fd5b50808373ffffffffffffffffffffffffffffffffffffffff167fa641bcd8a48e29cb86bb641e1ad9cb6642ccd0227d91ec198044193b7f8416b760405160405180910390a35b505050565b600047905090565b3373ffffffffffffffffffffffffffffffffffffffff1660008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16146103fb576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016103f290610686565b60405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff16141561046b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161046290610666565b60405180910390fd5b61047481610477565b50565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050816000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508173ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a35050565b60008135905061054a8161079d565b92915050565b60008135905061055f816107b4565b92915050565b600081359050610574816107cb565b92915050565b6000602082840312156105905761058f610720565b5b600061059e8482850161053b565b91505092915050565b600080604083850312156105be576105bd610720565b5b60006105cc85828601610550565b92505060206105dd85828601610565565b9150509250929050565b6105f0816106d2565b82525050565b60006106036026836106c1565b915061060e82610725565b604082019050919050565b6000610626601c836106c1565b915061063182610774565b602082019050919050565b61064581610716565b82525050565b600060208201905061066060008301846105e7565b92915050565b6000602082019050818103600083015261067f816105f6565b9050919050565b6000602082019050818103600083015261069f81610619565b9050919050565b60006020820190506106bb600083018461063c565b92915050565b600082825260208201905092915050565b60006106dd826106f6565b9050919050565b60006106ef826106f6565b9050919050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000819050919050565b600080fd5b7f4f776e61626c653a206e6577206f776e657220697320746865207a65726f206160008201527f6464726573730000000000000000000000000000000000000000000000000000602082015250565b7f4f6e6c79206f776e65722063616e2063616c6c2066756e6374696f6e00000000600082015250565b6107a6816106d2565b81146107b157600080fd5b50565b6107bd816106e4565b81146107c857600080fd5b50565b6107d481610716565b81146107df57600080fd5b5056fea264697066735822122068626461fb85e62e36e5d52befd5046fb9c47fb6879e39ace5994eb7694568f464736f6c63430008060033"
)

// PredeployVaultSC is a helper method for setting up the vault smart contract account,
// using the passed in owner and signers as pre-defined accounts.
func PredeployVaultSC(params PredeployParams) (*chain.GenesisAccount, error) {
	// Set the code for the smart contract
	// Code retrieved from https://github.com/dogechain-lab/jury-contracts
	scHex, _ := hex.DecodeHex(VaultSCBytecode)
	contractAccount := &chain.GenesisAccount{
		Code: scHex,
	}

	// Generate the empty account storage map
	storageMap := make(map[types.Hash]types.Hash)
	// Set the value for the owner
	storageIndexes := getStorageIndexes()
	storageMap[types.BytesToHash(storageIndexes.OwnerIndex)] =
		types.BytesToHash(params.Owner.Bytes())

	// Save the storage map
	contractAccount.Storage = storageMap

	return contractAccount, nil
}
