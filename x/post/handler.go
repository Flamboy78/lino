package post

import (
	"fmt"
	"reflect"

	"github.com/lino-network/lino/types"
	"github.com/lino-network/lino/x/global"

	sdk "github.com/cosmos/cosmos-sdk/types"
	acc "github.com/lino-network/lino/x/account"
	dev "github.com/lino-network/lino/x/developer"
	rep "github.com/lino-network/lino/x/reputation"
)

// NewHandler - Handle all "post" type messages.
func NewHandler(
	pm PostManager, am acc.AccountManager, gm *global.GlobalManager,
	dm dev.DeveloperManager, rm rep.ReputationManager) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		case CreatePostMsg:
			return handleCreatePostMsg(ctx, msg, pm, am, gm)
		case DonateMsg:
			return handleDonateMsg(ctx, msg, pm, am, gm, dm, rm)
		case ReportOrUpvoteMsg:
			return handleReportOrUpvoteMsg(ctx, msg, pm, am, gm, rm)
		case ViewMsg:
			return handleViewMsg(ctx, msg, pm, am, gm)
		case UpdatePostMsg:
			return handleUpdatePostMsg(ctx, msg, pm, am)
		case DeletePostMsg:
			return handleDeletePostMsg(ctx, msg, pm, am)
		default:
			errMsg := fmt.Sprintf("Unrecognized post msg type: %v", reflect.TypeOf(msg).Name())
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

// Handle RegisterMsg
func handleCreatePostMsg(ctx sdk.Context, msg CreatePostMsg, pm PostManager, am acc.AccountManager, gm *global.GlobalManager) sdk.Result {
	if !am.DoesAccountExist(ctx, msg.Author) {
		return ErrAccountNotFound(msg.Author).Result()
	}
	permlink := types.GetPermlink(msg.Author, msg.PostID)
	if pm.DoesPostExist(ctx, permlink) {
		return ErrPostAlreadyExist(permlink).Result()
	}
	postParam, err := pm.paramHolder.GetPostParam(ctx)
	if err != nil {
		return err.Result()
	}
	lastPostAt, err := am.GetLastPostAt(ctx, msg.Author)
	if err != nil {
		return err.Result()
	}
	if lastPostAt+postParam.PostIntervalSec > ctx.BlockHeader().Time.Unix() {
		return ErrPostTooOften(msg.Author).Result()
	}
	if len(msg.ParentAuthor) > 0 || len(msg.ParentPostID) > 0 {
		parentPostKey := types.GetPermlink(msg.ParentAuthor, msg.ParentPostID)
		if !pm.DoesPostExist(ctx, parentPostKey) {
			return ErrPostNotFound(parentPostKey).Result()
		}
		if err := pm.AddComment(ctx, parentPostKey, msg.Author, msg.PostID); err != nil {
			return err.Result()
		}
	}

	splitRate, err := sdk.NewDecFromStr(msg.RedistributionSplitRate)
	if err != nil {
		return ErrInvalidPostRedistributionSplitRate().Result()
	}

	if err := pm.CreatePost(
		ctx, msg.Author, msg.PostID, msg.SourceAuthor, msg.SourcePostID,
		msg.ParentAuthor, msg.ParentPostID, msg.Content, msg.Title,
		splitRate, msg.Links); err != nil {
		return err.Result()
	}

	if err := am.UpdateLastPostAt(ctx, msg.Author); err != nil {
		return err.Result()
	}
	return sdk.Result{}
}

// Handle ViewMsg
func handleViewMsg(ctx sdk.Context, msg ViewMsg, pm PostManager, am acc.AccountManager, gm *global.GlobalManager) sdk.Result {
	if !am.DoesAccountExist(ctx, msg.Username) {
		return ErrAccountNotFound(msg.Username).Result()
	}
	permlink := types.GetPermlink(msg.Author, msg.PostID)
	if !pm.DoesPostExist(ctx, permlink) {
		return ErrPostNotFound(permlink).Result()
	}
	if err := pm.AddOrUpdateViewToPost(ctx, permlink, msg.Username); err != nil {
		return err.Result()
	}

	return sdk.Result{}
}

// Handle DonateMsg
func handleDonateMsg(
	ctx sdk.Context, msg DonateMsg, pm PostManager, am acc.AccountManager,
	gm *global.GlobalManager, dm dev.DeveloperManager, rm rep.ReputationManager) sdk.Result {
	permlink := types.GetPermlink(msg.Author, msg.PostID)
	coin, err := types.LinoToCoin(msg.Amount)
	if err != nil {
		return err.Result()
	}
	if !pm.DoesPostExist(ctx, permlink) {
		return ErrPostNotFound(permlink).Result()
	}
	if isDeleted, err := pm.IsDeleted(ctx, permlink); isDeleted || err != nil {
		return ErrDonatePostIsDeleted(permlink).Result()
	}

	if msg.Username == msg.Author {
		return ErrCannotDonateToSelf(msg.Username).Result()
	}
	if msg.FromApp != "" {
		if !dm.DoesDeveloperExist(ctx, msg.FromApp) {
			return ErrDeveloperNotFound(msg.FromApp).Result()
		}
	}

	totalCoinDayDonated, err := am.MinusSavingCoinWithFullCoinDay(
		ctx, msg.Username, coin, msg.Author, msg.Memo,
		types.DonationOut)
	if err != nil {
		return err.Result()
	}

	// sourceAuthor, sourcePostID, err := pm.GetSourcePost(ctx, permlink)
	// if err != nil {
	// 	return err.Result()
	// }
	// if sourceAuthor != types.AccountKey("") && sourcePostID != "" {
	// 	sourcePermlink := types.GetPermlink(sourceAuthor, sourcePostID)

	// 	redistributionSplitRate, err := pm.GetRedistributionSplitRate(ctx, sourcePermlink)
	// 	if err != nil {
	// 		return err.Result()
	// 	}
	// 	sourceIncome := types.DecToCoin(coin.ToDec().Mul(sdk.OneDec().Sub(redistributionSplitRate)))
	// 	coin = coin.Minus(sourceIncome)
	// 	sourceCoinDayGained := types.DecToCoin(totalCoinDayDonated.ToDec().Mul(sdk.OneDec().Sub(redistributionSplitRate)))
	// 	totalCoinDayDonated = totalCoinDayDonated.Minus(sourceCoinDayGained)
	// 	if err := processDonationFriction(
	// 		ctx, msg.Username, sourceIncome, sourceCoinDayGained, sourceAuthor, sourcePostID, msg.FromApp, am, pm, gm, rm); err != nil {
	// 		return ErrProcessSourceDonation(sourcePermlink).Result()
	// 	}
	// }
	if err := processDonationFriction(
		ctx, msg.Username, coin, totalCoinDayDonated, msg.Author, msg.PostID, msg.FromApp, msg.Memo, am, pm, gm, rm); err != nil {
		return ErrProcessDonation(permlink).Result()
	}
	return sdk.Result{}
}

func processDonationFriction(
	ctx sdk.Context, consumer types.AccountKey, coin types.Coin, coinDayDonated types.Coin,
	postAuthor types.AccountKey, postID string, fromApp types.AccountKey, memo string, am acc.AccountManager,
	pm PostManager, gm *global.GlobalManager, rm rep.ReputationManager) sdk.Error {
	postKey := types.GetPermlink(postAuthor, postID)
	if coin.IsZero() {
		return nil
	}
	consumptionFrictionRate, err := gm.GetConsumptionFrictionRate(ctx)
	if err != nil {
		return err
	}
	frictionCoin := types.DecToCoin(coin.ToDec().Mul(consumptionFrictionRate))
	// evaluate this consumption can get the result, the result is used to get inflation from pool
	repInput := coinDayDonated
	if ctx.BlockHeader().Height >= types.BlockchainUpgrade1Update5Height {
		repInput = coin
	}
	dp, err := rm.DonateAt(ctx, consumer, postKey, repInput)
	if err != nil {
		return err
	}
	evaluateResult, err := evaluateConsumption(dp, gm)
	if err != nil {
		return err
	}
	rewardEvent := RewardEvent{
		PostAuthor: postAuthor,
		PostID:     postID,
		Consumer:   consumer,
		Evaluate:   evaluateResult,
		Original:   coin,
		Friction:   frictionCoin,
		FromApp:    fromApp,
	}
	if err := gm.AddFrictionAndRegisterContentRewardEvent(
		ctx, rewardEvent, frictionCoin, evaluateResult); err != nil {
		return err
	}

	directDeposit := coin.Minus(frictionCoin)
	if err := pm.AddDonation(ctx, postKey, consumer, directDeposit, types.DirectDeposit); err != nil {
		return err
	}
	if err := am.AddSavingCoin(
		ctx, postAuthor, directDeposit, consumer, memo, types.DonationIn); err != nil {
		return err
	}
	if err := am.AddDirectDeposit(ctx, postAuthor, directDeposit); err != nil {
		return err
	}
	if err := gm.AddConsumption(ctx, coin); err != nil {
		return err
	}
	return nil
}

// XXX(yumin): deprecated, chained on gm.EvaluateConsumption
func evaluateConsumption(coin types.Coin, gm *global.GlobalManager) (types.Coin, sdk.Error) {
	return gm.EvaluateConsumption(coin)
}

// Handle ReportMsgOrUpvoteMsg
func handleReportOrUpvoteMsg(
	ctx sdk.Context, msg ReportOrUpvoteMsg, pm PostManager, am acc.AccountManager,
	gm *global.GlobalManager, rm rep.ReputationManager) sdk.Result {
	if !am.DoesAccountExist(ctx, msg.Username) {
		return ErrAccountNotFound(msg.Username).Result()
	}

	permlink := types.GetPermlink(msg.Author, msg.PostID)
	if !pm.DoesPostExist(ctx, permlink) {
		return ErrPostNotFound(permlink).Result()
	}

	postParam, err := pm.paramHolder.GetPostParam(ctx)
	if err != nil {
		return err.Result()
	}

	lastReportOrUpvoteAt, err := am.GetLastReportOrUpvoteAt(ctx, msg.Username)
	if err != nil {
		return err.Result()
	}

	if lastReportOrUpvoteAt+postParam.ReportOrUpvoteIntervalSec > ctx.BlockHeader().Time.Unix() {
		return ErrReportOrUpvoteTooOften().Result()
	}
	if msg.IsReport {
		if _, err := rm.ReportAt(ctx, msg.Username, permlink); err != nil {
			return err.Result()
		}
	}
	if err := pm.UpdateLastActivityAt(ctx, permlink); err != nil {
		return err.Result()
	}

	if err := am.UpdateLastReportOrUpvoteAt(ctx, msg.Username); err != nil {
		return err.Result()
	}
	return sdk.Result{}
}

func handleUpdatePostMsg(
	ctx sdk.Context, msg UpdatePostMsg, pm PostManager, am acc.AccountManager) sdk.Result {
	if !am.DoesAccountExist(ctx, msg.Author) {
		return ErrAccountNotFound(msg.Author).Result()
	}
	permlink := types.GetPermlink(msg.Author, msg.PostID)
	if !pm.DoesPostExist(ctx, permlink) {
		return ErrPostNotFound(permlink).Result()
	}
	if isDeleted, err := pm.IsDeleted(ctx, permlink); isDeleted || err != nil {
		return ErrUpdatePostIsDeleted(permlink).Result()
	}

	if err := pm.UpdatePost(
		ctx, msg.Author, msg.PostID, msg.Title, msg.Content, msg.Links); err != nil {
		return err.Result()
	}
	return sdk.Result{}
}

func handleDeletePostMsg(
	ctx sdk.Context, msg DeletePostMsg, pm PostManager, am acc.AccountManager) sdk.Result {
	if !am.DoesAccountExist(ctx, msg.Author) {
		return ErrAccountNotFound(msg.Author).Result()
	}
	permlink := types.GetPermlink(msg.Author, msg.PostID)
	if !pm.DoesPostExist(ctx, permlink) {
		return ErrPostNotFound(permlink).Result()
	}

	if err := pm.DeletePost(ctx, permlink); err != nil {
		return err.Result()
	}
	return sdk.Result{}
}
