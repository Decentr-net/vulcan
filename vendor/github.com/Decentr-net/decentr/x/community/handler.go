package community

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/gofrs/uuid"
)

// NewHandler creates an sdk.Handler for all the community type messages
func NewHandler(keeper Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		switch msg := msg.(type) {
		case MsgCreatePost:
			return handleMsgCreatePost(ctx, keeper, msg)
		case MsgDeletePost:
			return handleMsgDeletePost(ctx, keeper, msg)
		case MsgSetLike:
			return handleMsgSetLike(ctx, keeper, msg)
		case MsgFollow:
			return handleMsgFollow(ctx, keeper, msg)
		case MsgUnfollow:
			return handleMsgUnfollow(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("unrecognized %s message type: %T", ModuleName, msg)
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}

func handleMsgCreatePost(ctx sdk.Context, keeper Keeper, msg MsgCreatePost) (*sdk.Result, error) {
	id, _ := uuid.FromString(msg.UUID)
	keeper.CreatePost(ctx, Post{
		UUID:         id,
		Owner:        msg.Owner,
		Title:        msg.Title,
		Category:     msg.Category,
		PreviewImage: msg.PreviewImage,
		Text:         msg.Text,
	})

	return &sdk.Result{}, nil
}

func handleMsgDeletePost(ctx sdk.Context, keeper Keeper, msg MsgDeletePost) (*sdk.Result, error) {
	moderators := keeper.GetModerators(ctx)
	var isModerator bool
	for _, moderator := range moderators {
		addr, _ := sdk.AccAddressFromBech32(moderator)
		if msg.Owner.Equals(addr) && !addr.Empty() {
			isModerator = true
			break
		}
	}

	if !isModerator && !msg.Owner.Equals(msg.PostOwner) {
		return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "Incorrect Owner")
	}

	postUUID, _ := uuid.FromString(msg.PostUUID)
	keeper.DeletePost(ctx, msg.PostOwner, postUUID)

	if isModerator {
		ctx.Logger().Info("moderator deleted post %s %s", msg.PostOwner, msg.PostUUID)
	}

	return &sdk.Result{}, nil
}

func handleMsgSetLike(ctx sdk.Context, keeper Keeper, msg MsgSetLike) (*sdk.Result, error) {
	postUUID, _ := uuid.FromString(msg.PostUUID)
	keeper.SetLike(ctx, Like{
		PostOwner: msg.PostOwner,
		PostUUID:  postUUID,
		Owner:     msg.Owner,
		Weight:    msg.Weight,
	})
	return &sdk.Result{}, nil
}

func handleMsgFollow(ctx sdk.Context, keeper Keeper, msg MsgFollow) (*sdk.Result, error) {
	if msg.Owner.Equals(msg.Whom) {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "Owner cannot follow himself")
	}

	keeper.Follow(ctx, msg.Owner, msg.Whom)
	return &sdk.Result{}, nil
}

func handleMsgUnfollow(ctx sdk.Context, keeper Keeper, msg MsgUnfollow) (*sdk.Result, error) {
	keeper.Unfollow(ctx, msg.Owner, msg.Whom)
	return &sdk.Result{}, nil
}
