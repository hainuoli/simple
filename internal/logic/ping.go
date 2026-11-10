package logic

import (
	"context"
	"simple/internal/svc"
)

type PingLogic struct {
	ctx context.Context
	svc *svc.ServiceContext
}

func NewPingLogic(ctx context.Context, svc *svc.ServiceContext) *PingLogic {
	return &PingLogic{
		ctx: ctx,
		svc: svc,
	}
}

func (l *PingLogic) Ping() string {
	return "pong"
}
