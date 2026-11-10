package svc

import "simple/internal/config"

type ServiceContext struct {
	Conf config.Conf
}

func New(c config.Conf) *ServiceContext {
	return &ServiceContext{
		Conf: c,
	}
}
