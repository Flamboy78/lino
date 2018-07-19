package validator

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/lino-network/lino/types"
	"github.com/lino-network/lino/x/validator/model"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestByzantines(t *testing.T) {
	ctx, am, valManager, voteManager, gm := setupTest(t, 0)
	handler := NewHandler(am, valManager, voteManager, gm)
	valManager.InitGenesis(ctx)

	valParam, _ := valManager.paramHolder.GetValidatorParam(ctx)
	minBalance := types.NewCoinFromInt64(100000 * types.Decimals)
	// create 21 test users
	users := make([]types.AccountKey, 21)
	valKeys := make([]crypto.PubKey, 21)
	for i := 0; i < 21; i++ {
		users[i] = createTestAccount(ctx, am, "user"+strconv.Itoa(i), minBalance.Plus(valParam.ValidatorMinCommitingDeposit))
		voteManager.AddVoter(ctx, types.AccountKey("user"+strconv.Itoa(i)), valParam.ValidatorMinVotingDeposit)

		// they will deposit 10,20,30...200, 210
		num := int64((i+1)*10) + valParam.ValidatorMinCommitingDeposit.ToInt64()/types.Decimals
		deposit := types.LNO(strconv.FormatInt(num, 10))
		valKeys[i] = crypto.GenPrivKeyEd25519().PubKey()
		name := "user" + strconv.Itoa(i)
		msg := NewValidatorDepositMsg(name, deposit, valKeys[i], "")
		result := handler(ctx, msg)
		assert.Equal(t, sdk.Result{}, result)
	}

	// byzantine
	byzantineList := []int32{3, 8, 14}
	byzantines := []abci.Evidence{}
	for _, idx := range byzantineList {
		byzantines = append(byzantines, abci.Evidence{Validator: abci.Validator{
			Address: valKeys[idx].Address(),
			PubKey:  tmtypes.TM2PB.PubKey(valKeys[idx]),
			Power:   1000}})
	}
	_, err := valManager.FireIncompetentValidator(ctx, byzantines)
	assert.Nil(t, err)

	validatorList3, _ := valManager.storage.GetValidatorList(ctx)
	assert.Equal(t, 18, len(validatorList3.OncallValidators))
	assert.Equal(t, 18, len(validatorList3.AllValidators))

	for _, idx := range byzantineList {
		assert.Equal(t, -1, FindAccountInList(users[idx], validatorList3.OncallValidators))
		assert.Equal(t, -1, FindAccountInList(users[idx], validatorList3.AllValidators))
	}

}

func TestAbsentValidator(t *testing.T) {
	ctx, am, valManager, voteManager, gm := setupTest(t, 0)
	handler := NewHandler(am, valManager, voteManager, gm)
	valManager.InitGenesis(ctx)

	valParam, _ := valManager.paramHolder.GetValidatorParam(ctx)
	minBalance := types.NewCoinFromInt64(100000 * types.Decimals)
	// create 21 test users
	users := make([]types.AccountKey, 21)
	valKeys := make([]crypto.PubKey, 21)
	for i := 0; i < 21; i++ {
		users[i] = createTestAccount(ctx, am, "user"+strconv.Itoa(i), minBalance.Plus(valParam.ValidatorMinCommitingDeposit))
		voteManager.AddVoter(ctx, types.AccountKey("user"+strconv.Itoa(i)), valParam.ValidatorMinVotingDeposit)

		// they will deposit 10,20,30...200, 210
		num := int64((i+1)*10) + valParam.ValidatorMinCommitingDeposit.ToInt64()/types.Decimals
		deposit := types.LNO(strconv.FormatInt(num, 10))
		valKeys[i] = crypto.GenPrivKeyEd25519().PubKey()
		name := "user" + strconv.Itoa(i)
		msg := NewValidatorDepositMsg(name, deposit, valKeys[i], "")
		result := handler(ctx, msg)
		assert.Equal(t, sdk.Result{}, result)

	}

	// construct signing list
	absentList := []int{0, 1, 10}
	index := 0
	signingList := []abci.SigningValidator{}
	for i := 0; i < 21; i++ {
		signingList = append(signingList, abci.SigningValidator{
			Validator: abci.Validator{
				Address: valKeys[i].Address(),
				PubKey:  tmtypes.TM2PB.PubKey(valKeys[i]),
				Power:   1000},
			SignedLastBlock: true,
		})
		if index < len(absentList) && i == absentList[index] {
			signingList[i].SignedLastBlock = false
			index++
		}
	}
	// shuffle the signing validator array
	destSigningList := make([]abci.SigningValidator, len(signingList))
	perm := rand.Perm(len(signingList))
	for i, v := range perm {
		destSigningList[v] = signingList[i]
	}
	err := valManager.UpdateSigningValidator(ctx, signingList)
	assert.Nil(t, err)

	index = 0
	for i := 0; i < 21; i++ {
		validator, _ := valManager.storage.GetValidator(ctx, types.AccountKey("user"+strconv.Itoa(i)))
		if index < len(absentList) && i == absentList[index] {
			assert.Equal(t, int64(1), validator.AbsentCommit)
			assert.Equal(t, int64(0), validator.ProducedBlocks)
			index++
		} else {
			assert.Equal(t, int64(0), validator.AbsentCommit)
			assert.Equal(t, int64(1), validator.ProducedBlocks)
		}
	}

	param, _ := valManager.paramHolder.GetValidatorParam(ctx)
	// absent exceeds limitation
	for i := int64(0); i < param.AbsentCommitLimitation; i++ {
		err := valManager.UpdateSigningValidator(ctx, signingList)
		assert.Nil(t, err)
	}

	index = 0
	for i := 0; i < 21; i++ {
		validator, _ := valManager.storage.GetValidator(ctx, types.AccountKey("user"+strconv.Itoa(i)))
		if index < len(absentList) && i == absentList[index] {
			assert.Equal(t, int64(101), validator.AbsentCommit)
			assert.Equal(t, int64(0), validator.ProducedBlocks)
			index++
		} else {
			assert.Equal(t, int64(0), validator.AbsentCommit)
			assert.Equal(t, int64(101), validator.ProducedBlocks)
		}
	}

	_, err = valManager.FireIncompetentValidator(ctx, []abci.Evidence{})
	assert.Nil(t, err)
	validatorList2, _ := valManager.storage.GetValidatorList(ctx)
	assert.Equal(t, 18, len(validatorList2.OncallValidators))
	assert.Equal(t, 18, len(validatorList2.AllValidators))

	for _, idx := range absentList {
		assert.Equal(t, -1, FindAccountInList(types.AccountKey("user"+strconv.Itoa(idx)), validatorList2.OncallValidators))
		assert.Equal(t, -1, FindAccountInList(types.AccountKey("user"+strconv.Itoa(idx)), validatorList2.AllValidators))
	}

}

func TestGetOncallList(t *testing.T) {
	ctx, am, valManager, voteManager, gm := setupTest(t, 0)
	handler := NewHandler(am, valManager, voteManager, gm)
	valManager.InitGenesis(ctx)

	valParam, _ := valManager.paramHolder.GetValidatorParam(ctx)
	minBalance := types.NewCoinFromInt64(100000 * types.Decimals)
	// create 21 test users
	users := make([]types.AccountKey, 21)
	valKeys := make([]crypto.PubKey, 21)
	for i := 0; i < 21; i++ {
		users[i] = createTestAccount(ctx, am, "user"+strconv.Itoa(i), minBalance.Plus(valParam.ValidatorMinCommitingDeposit))
		voteManager.AddVoter(ctx, types.AccountKey("user"+strconv.Itoa(i)), valParam.ValidatorMinVotingDeposit)

		// they will deposit 10,20,30...200, 210
		num := int64((i+1)*10) + valParam.ValidatorMinCommitingDeposit.ToInt64()/types.Decimals
		deposit := types.LNO(strconv.FormatInt(num, 10))
		valKeys[i] = crypto.GenPrivKeyEd25519().PubKey()
		name := "user" + strconv.Itoa(i)
		msg := NewValidatorDepositMsg(name, deposit, valKeys[i], "")
		result := handler(ctx, msg)
		assert.Equal(t, sdk.Result{}, result)
	}

	lst, _ := valManager.GetValidatorList(ctx)
	for idx, validator := range lst.OncallValidators {
		assert.Equal(t, users[idx], validator)
	}

}

func TestPunishmentBasic(t *testing.T) {
	ctx, am, valManager, voteManager, gm := setupTest(t, 0)
	handler := NewHandler(am, valManager, voteManager, gm)
	valManager.InitGenesis(ctx)

	valParam, _ := valManager.paramHolder.GetValidatorParam(ctx)

	minBalance := types.NewCoinFromInt64(1 * types.Decimals)
	createTestAccount(ctx, am, "user1", minBalance.Plus(valParam.ValidatorMinCommitingDeposit))
	createTestAccount(ctx, am, "user2", minBalance.Plus(valParam.ValidatorMinCommitingDeposit))

	valKey1 := crypto.GenPrivKeyEd25519().PubKey()
	valKey2 := crypto.GenPrivKeyEd25519().PubKey()

	voteManager.AddVoter(ctx, "user1", valParam.ValidatorMinVotingDeposit)
	voteManager.AddVoter(ctx, "user2", valParam.ValidatorMinVotingDeposit)

	// let both users register as validator
	msg1 := NewValidatorDepositMsg("user1", coinToString(valParam.ValidatorMinCommitingDeposit), valKey1, "")
	msg2 := NewValidatorDepositMsg("user2", coinToString(valParam.ValidatorMinCommitingDeposit), valKey2, "")
	handler(ctx, msg1)
	handler(ctx, msg2)

	// punish user2 as byzantine (explicitly remove)
	valManager.PunishOncallValidator(ctx, types.AccountKey("user2"), valParam.PenaltyByzantine, true)
	lst, _ := valManager.storage.GetValidatorList(ctx)
	assert.Equal(t, 1, len(lst.OncallValidators))
	assert.Equal(t, 1, len(lst.AllValidators))
	assert.Equal(t, types.AccountKey("user1"), lst.OncallValidators[0])

	validator, _ := valManager.storage.GetValidator(ctx, "user2")
	assert.Equal(t, true, validator.Deposit.IsZero())

	// punish user1 as missing vote (wont explicitly remove)
	valManager.PunishOncallValidator(ctx, types.AccountKey("user1"), valParam.PenaltyMissVote, false)
	lst2, _ := valManager.storage.GetValidatorList(ctx)
	assert.Equal(t, 0, len(lst2.OncallValidators))
	assert.Equal(t, 0, len(lst2.AllValidators))

	validator2, _ := valManager.storage.GetValidator(ctx, "user1")
	assert.Equal(t, true, validator2.Deposit.IsZero())
}

func TestPunishmentAndSubstitutionExists(t *testing.T) {
	ctx, am, valManager, voteManager, gm := setupTest(t, 0)
	handler := NewHandler(am, valManager, voteManager, gm)
	valManager.InitGenesis(ctx)

	valParam, _ := valManager.paramHolder.GetValidatorParam(ctx)
	minBalance := types.NewCoinFromInt64(100000 * types.Decimals)

	// create 24 test users
	users := make([]types.AccountKey, 24)
	valKeys := make([]crypto.PubKey, 24)
	for i := 0; i < 24; i++ {
		users[i] = createTestAccount(ctx, am, "user"+strconv.Itoa(i+1), minBalance.Plus(valParam.ValidatorMinCommitingDeposit))
		voteManager.AddVoter(ctx, types.AccountKey("user"+strconv.Itoa(i+1)), valParam.ValidatorMinVotingDeposit)

		num := int64((i+1)*1000) + valParam.ValidatorMinCommitingDeposit.ToInt64()/types.Decimals
		deposit := types.LNO(strconv.FormatInt(num, 10))

		valKeys[i] = crypto.GenPrivKeyEd25519().PubKey()
		msg := NewValidatorDepositMsg("user"+strconv.Itoa(i+1), deposit, valKeys[i], "")
		result := handler(ctx, msg)
		assert.Equal(t, sdk.Result{}, result)
	}

	// lowest is user4 with power (min + 400)
	lst, _ := valManager.storage.GetValidatorList(ctx)
	assert.Equal(t, 21, len(lst.OncallValidators))
	assert.Equal(t, 24, len(lst.AllValidators))
	assert.Equal(t, valParam.ValidatorMinCommitingDeposit.Plus(types.NewCoinFromInt64(4000*types.Decimals)), lst.LowestPower)
	assert.Equal(t, users[3], lst.LowestValidator)

	// punish user4 as missing vote (wont explicitly remove)
	// user3 will become the lowest one with power (min + 3000)
	valManager.PunishOncallValidator(ctx, users[3], types.NewCoinFromInt64(2000*types.Decimals), false)
	lst2, _ := valManager.storage.GetValidatorList(ctx)
	assert.Equal(t, 21, len(lst2.OncallValidators))
	assert.Equal(t, 24, len(lst2.AllValidators))
	assert.Equal(t, valParam.ValidatorMinCommitingDeposit.Plus(types.NewCoinFromInt64(3000*types.Decimals)), lst2.LowestPower)
	assert.Equal(t, users[2], lst2.LowestValidator)

}

func TestGetUpdateValidatorList(t *testing.T) {
	ctx, am, valManager, _, _ := setupTest(t, 0)
	valManager.InitGenesis(ctx)

	minBalance := types.NewCoinFromInt64(100 * types.Decimals)

	user1 := createTestAccount(ctx, am, "user1", minBalance)
	user2 := createTestAccount(ctx, am, "user2", minBalance)

	valKey1 := crypto.GenPrivKeyEd25519().PubKey()
	valKey2 := crypto.GenPrivKeyEd25519().PubKey()

	param, _ := valManager.paramHolder.GetValidatorParam(ctx)

	valManager.RegisterValidator(ctx, user1, valKey1, param.ValidatorMinCommitingDeposit, "")
	valManager.RegisterValidator(ctx, user2, valKey2, param.ValidatorMinCommitingDeposit, "")

	val1, _ := valManager.storage.GetValidator(ctx, user1)
	val2, _ := valManager.storage.GetValidator(ctx, user2)

	val1NoPower := abci.Validator{
		Address: valKey1.Address(),
		Power:   0,
		PubKey:  val1.ABCIValidator.GetPubKey(),
	}

	val2NoPower := abci.Validator{
		Address: valKey2.Address(),
		Power:   0,
		PubKey:  val2.ABCIValidator.GetPubKey(),
	}

	testCases := []struct {
		testName            string
		oncallValidators    []types.AccountKey
		preBlockValidators  []types.AccountKey
		expectedUpdatedList []abci.Validator
	}{
		{
			testName:            "only one oncall validator",
			oncallValidators:    []types.AccountKey{user1},
			preBlockValidators:  []types.AccountKey{},
			expectedUpdatedList: []abci.Validator{val1.ABCIValidator},
		},
		{
			testName:            "two oncall validators and one pre block validator",
			oncallValidators:    []types.AccountKey{user1, user2},
			preBlockValidators:  []types.AccountKey{user1},
			expectedUpdatedList: []abci.Validator{val1.ABCIValidator, val2.ABCIValidator},
		},
		{
			testName:            "two oncall validatos and two pre block validators",
			oncallValidators:    []types.AccountKey{user1, user2},
			preBlockValidators:  []types.AccountKey{user1, user2},
			expectedUpdatedList: []abci.Validator{val1.ABCIValidator, val2.ABCIValidator},
		},
		{
			testName:            "one oncall validator and two pre block validators",
			oncallValidators:    []types.AccountKey{user2},
			preBlockValidators:  []types.AccountKey{user1, user2},
			expectedUpdatedList: []abci.Validator{val1NoPower, val2.ABCIValidator},
		},
		{
			testName:            "only one pre block validator",
			oncallValidators:    []types.AccountKey{},
			preBlockValidators:  []types.AccountKey{user2},
			expectedUpdatedList: []abci.Validator{val2NoPower},
		},
	}

	for _, tc := range testCases {
		lst := &model.ValidatorList{
			OncallValidators:   tc.oncallValidators,
			PreBlockValidators: tc.preBlockValidators,
		}
		err := valManager.storage.SetValidatorList(ctx, lst)
		if err != nil {
			t.Errorf("%s: failed to set validator list, got err %v", tc.testName, err)
		}

		actualList, err := valManager.GetUpdateValidatorList(ctx)
		if err != nil {
			t.Errorf("%s: failed to get validator list, got err %v", tc.testName, err)
		}
		if !assert.Equal(t, tc.expectedUpdatedList, actualList) {
			t.Errorf("%s: diff result, got %v, want %v", tc.testName, actualList, tc.expectedUpdatedList)
		}
	}
}

func TestIsLegalWithdraw(t *testing.T) {
	ctx, am, valManager, _, _ := setupTest(t, 0)
	minBalance := types.NewCoinFromInt64(100 * types.Decimals)

	user1 := createTestAccount(ctx, am, "user1", minBalance)
	param, _ := valManager.paramHolder.GetValidatorParam(ctx)
	valManager.InitGenesis(ctx)
	valManager.RegisterValidator(
		ctx, user1, crypto.GenPrivKeyEd25519().PubKey(),
		param.ValidatorMinCommitingDeposit.Plus(types.NewCoinFromInt64(100*types.Decimals)), "")

	testCases := []struct {
		testName         string
		oncallValidators []types.AccountKey
		username         types.AccountKey
		withdraw         types.Coin
		expectedResult   bool
	}{
		{
			testName:         "withdraw amount is a little less than minimum requirement",
			oncallValidators: []types.AccountKey{user1},
			username:         user1,
			withdraw:         param.ValidatorMinWithdraw.Minus(types.NewCoinFromInt64(1)),
			expectedResult:   false,
		},
		{
			testName:         "remaing coin is less than minimum commiting deposit",
			oncallValidators: []types.AccountKey{user1},
			username:         user1,
			withdraw:         param.ValidatorMinCommitingDeposit,
			expectedResult:   false,
		},
		{
			testName:         "oncall validator can't withdraw",
			oncallValidators: []types.AccountKey{user1},
			username:         user1,
			withdraw:         param.ValidatorMinWithdraw,
			expectedResult:   false,
		},
		{
			testName:         "withdraw successfully",
			oncallValidators: []types.AccountKey{},
			username:         user1,
			withdraw:         param.ValidatorMinWithdraw,
			expectedResult:   true,
		},
	}

	for _, tc := range testCases {
		lst := &model.ValidatorList{
			OncallValidators: tc.oncallValidators,
		}
		err := valManager.storage.SetValidatorList(ctx, lst)
		if err != nil {
			t.Errorf("%s: failed to set validator list, got err %v", tc.testName, err)
		}

		res := valManager.IsLegalWithdraw(ctx, tc.username, tc.withdraw)
		if res != tc.expectedResult {
			t.Errorf("%s: diff result, got %v, want %v", tc.testName, res, tc.expectedResult)
		}
	}
}
