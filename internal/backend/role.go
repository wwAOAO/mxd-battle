package backend

import (
	"context"
	"fmt"
	"time"

	rolev1 "mxd-battle/internal/gen/mxd/role/v1"
)

type Role struct {
	ID           string
	AccountID    string
	Nickname     string
	Level        int32
	Exp          string
	JobCode      string
	Strength     int32
	Intelligence int32
	Agility      int32
	Luck         int32
	HP           int32
	MP           int32
	HPMax        int32
	MPMax        int32
	MapID        int32
	X            float64
	Y            float64
}

func (c *Client) GetAccountRole(ctx context.Context, accountID string, roleID string) (Role, error) {
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	response, err := c.Role.GetAccountRole(callCtx, &rolev1.GetAccountRoleRequest{
		AccountId: accountID,
		RoleId:    roleID,
	})
	if err != nil {
		return Role{}, fmt.Errorf("get account role: %w", err)
	}

	return Role{
		ID:           response.GetId(),
		AccountID:    response.GetAccountId(),
		Nickname:     response.GetNickname(),
		Level:        response.GetLevel(),
		Exp:          response.GetExp(),
		Strength:     response.GetStrength(),
		Intelligence: response.GetIntelligence(),
		Agility:      response.GetAgility(),
		Luck:         response.GetLuck(),
		HP:           response.GetHp(),
		MP:           response.GetMp(),
		HPMax:        response.GetHpMax(),
		MPMax:        response.GetMpMax(),
		MapID:        response.GetMapId(),
		X:            response.GetX(),
		Y:            response.GetY(),
	}, nil
}
