package validator

import (
	"strconv"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	acc "github.com/lino-network/lino/tx/account"
	"github.com/lino-network/lino/types"
	"github.com/stretchr/testify/assert"
)

var (
	c0    = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 0}}
	c10   = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 10}}
	c11   = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 11}}
	c20   = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 20}}
	c21   = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 21}}
	c100  = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 100}}
	c200  = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 200}}
	c400  = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 400}}
	c1000 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1000}}
	c1011 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1011}}
	c1021 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1021}}
	c1022 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1022}}
	c1600 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1600}}
	c1800 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1800}}
	c1900 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1900}}
	c2000 = sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 2000}}
)

func TestRegisterBasic(t *testing.T) {
	lam := newLinoAccountManager()
	vm := newValidatorManager()
	ctx := getContext()
	handler := NewHandler(vm, lam)
	vm.Init(ctx)

	// create two test users
	acc1 := createTestAccount(ctx, lam, "user1")
	acc1.AddCoins(ctx, c2000)
	acc1.Apply(ctx)

	// let user1 register as validator
	ownerKey, _ := acc1.GetOwnerKey(ctx)
	msg := NewValidatorDepositMsg("user1", c1600, *ownerKey)
	result := handler(ctx, msg)
	assert.Equal(t, sdk.Result{}, result)

	// check acc1's money has been withdrawn
	acc1Balance, _ := acc1.GetBankBalance(ctx)
	assert.Equal(t, acc1Balance, c400)
	assert.Equal(t, true, vm.IsValidatorExist(ctx, acc.AccountKey("user1")))

	// now user1 should be the only validator (WOW, dictator!)
	verifyList, _ := vm.GetValidatorList(ctx)
	assert.Equal(t, verifyList.LowestPower, c1600)
	assert.Equal(t, 1, len(verifyList.OncallValidators))
	assert.Equal(t, 1, len(verifyList.AllValidators))
	assert.Equal(t, acc.AccountKey("user1"), verifyList.OncallValidators[0])
	assert.Equal(t, acc.AccountKey("user1"), verifyList.AllValidators[0])

	// make sure the validator's account info (power&pubKey) is correct
	verifyAccount, _ := vm.GetValidator(ctx, acc.AccountKey("user1"))
	assert.Equal(t, int64(1600), verifyAccount.ABCIValidator.GetPower())
	assert.Equal(t, ownerKey.Bytes(), verifyAccount.ABCIValidator.GetPubKey())
}

func TestValidatorReplacement(t *testing.T) {
	lam := newLinoAccountManager()
	vm := newValidatorManager()
	ctx := getContext()
	handler := NewHandler(vm, lam)
	vm.Init(ctx)

	// create 21 test users
	users := make([]*acc.Account, 21)
	for i := 0; i < 21; i++ {
		users[i] = createTestAccount(ctx, lam, "user"+strconv.Itoa(i))
		users[i].AddCoins(ctx, c2000)
		users[i].Apply(ctx)
		// they will deposit 10,20,30...200, 210
		deposit := sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: int64((i+1)*10) + int64(1001)}}
		ownerKey, _ := users[i].GetOwnerKey(ctx)
		msg := NewValidatorDepositMsg("user"+strconv.Itoa(i), deposit, *ownerKey)
		result := handler(ctx, msg)
		assert.Equal(t, sdk.Result{}, result)
	}

	// check validator list, the lowest power is 10
	verifyList, _ := vm.GetValidatorList(ctx)
	assert.Equal(t, true, verifyList.LowestPower.IsEqual(c1011))
	assert.Equal(t, acc.AccountKey("user0"), verifyList.LowestValidator)
	assert.Equal(t, 21, len(verifyList.OncallValidators))
	assert.Equal(t, 21, len(verifyList.AllValidators))

	// create a user failed to join oncall validator list (not enough power)
	acc1 := createTestAccount(ctx, lam, "noPowerUser")
	acc1.AddCoins(ctx, c2000)
	acc1.Apply(ctx)

	//check the user hasn't been added to oncall validators but in the pool
	deposit := sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1005}}
	ownerKey1, _ := acc1.GetOwnerKey(ctx)
	msg := NewValidatorDepositMsg("noPowerUser", deposit, *ownerKey1)
	result := handler(ctx, msg)

	verifyList2, _ := vm.GetValidatorList(ctx)
	assert.Equal(t, sdk.Result{}, result)
	assert.Equal(t, true, verifyList.LowestPower.IsEqual(c1011))
	assert.Equal(t, acc.AccountKey("user0"), verifyList.LowestValidator)
	assert.Equal(t, 21, len(verifyList2.OncallValidators))
	assert.Equal(t, 22, len(verifyList2.AllValidators))

	// create a user success to join oncall validator list
	acc2 := createTestAccount(ctx, lam, "powerfulUser")
	acc2.AddCoins(ctx, c2000)
	acc2.Apply(ctx)

	//check the user has been added to oncall validators and in the pool
	deposit2 := sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1088}}
	ownerKey2, _ := acc2.GetOwnerKey(ctx)
	msg2 := NewValidatorDepositMsg("powerfulUser", deposit2, *ownerKey2)
	result2 := handler(ctx, msg2)

	verifyList3, _ := vm.GetValidatorList(ctx)
	assert.Equal(t, sdk.Result{}, result2)
	assert.Equal(t, true, verifyList3.LowestPower.IsEqual(c1021))
	assert.Equal(t, acc.AccountKey("user1"), verifyList3.LowestValidator)
	assert.Equal(t, 21, len(verifyList3.OncallValidators))
	assert.Equal(t, 23, len(verifyList3.AllValidators))

	// check user0 has been replaced, and powerful user has been added
	flag := false
	for _, username := range verifyList3.OncallValidators {
		if username == "powerfulUser" {
			flag = true
		}
		if username == "user0" {
			assert.Fail(t, "User0 should have been replaced")
		}
	}
	if !flag {
		assert.Fail(t, "Powerful user should have been added")
	}
}

func TestRemoveBasic(t *testing.T) {
	lam := newLinoAccountManager()
	vm := newValidatorManager()
	ctx := getContext()
	handler := NewHandler(vm, lam)
	vm.Init(ctx)

	// create two test users
	acc1 := createTestAccount(ctx, lam, "goodUser")
	ownerKey1, _ := acc1.GetOwnerKey(ctx)
	acc2 := createTestAccount(ctx, lam, "badUser")
	ownerKey2, _ := acc2.GetOwnerKey(ctx)
	acc1.AddCoins(ctx, c2000)
	acc1.Apply(ctx)
	acc2.AddCoins(ctx, c2000)
	acc2.Apply(ctx)

	// let both users register as validator
	deposit := sdk.Coins{sdk.Coin{Denom: types.Denom, Amount: 1200}}
	msg1 := NewValidatorDepositMsg("goodUser", deposit, *ownerKey1)
	msg2 := NewValidatorDepositMsg("badUser", deposit, *ownerKey2)
	handler(ctx, msg1)
	handler(ctx, msg2)

	verifyList, _ := vm.GetValidatorList(ctx)
	assert.Equal(t, 2, len(verifyList.OncallValidators))
	assert.Equal(t, 2, len(verifyList.AllValidators))

	vm.RemoveValidatorFromAllLists(ctx, "badUser")
	verifyList2, _ := vm.GetValidatorList(ctx)
	assert.Equal(t, 1, len(verifyList2.OncallValidators))
	assert.Equal(t, 1, len(verifyList2.AllValidators))
	assert.Equal(t, acc.AccountKey("goodUser"), verifyList2.OncallValidators[0])
	assert.Equal(t, acc.AccountKey("goodUser"), verifyList2.AllValidators[0])
}
