package system

import (
	"github.com/opskat/opskat/internal/app/i18n"
	"github.com/opskat/opskat/internal/service/jev_svc"
)

func (s *System) GetJevAPIKey() (string, error) { return jev_svc.APIKey(i18n.Ctx(s.ctx, s.Lang())) }

func (s *System) GetJevPrimaryConfidenceThreshold() float64 {
	return jev_svc.PrimaryConfidenceThreshold()
}

func (s *System) SaveJevAPIKey(key string) error {
	return jev_svc.SaveAPIKey(i18n.Ctx(s.ctx, s.Lang()), key)
}

func (s *System) SaveJevPrimaryConfidenceThreshold(threshold float64) error {
	return jev_svc.SavePrimaryConfidenceThreshold(i18n.Ctx(s.ctx, s.Lang()), threshold)
}
