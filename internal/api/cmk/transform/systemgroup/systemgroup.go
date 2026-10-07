package systemgroup

import (
	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/api/cmk/transform/system"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/sanitise"
)

func ToAPI(s *model.SystemGroup) (*cmkapi.SystemGroup, error) {
	err := sanitise.Sanitize(&s)
	if err != nil {
		return nil, err
	}

	apiSystems := make([]cmkapi.System, 0, len(s.Systems))
	for _, s := range s.Systems {
		apiSystem, err := system.ToAPI(*s, nil)
		if err != nil {
			return nil, err
		}
		apiSystems = append(apiSystems, *apiSystem)
	}
	return &cmkapi.SystemGroup{
		Description:     &s.Description,
		Id:              &s.ID,
		Name:            s.Name,
		SuppressWarning: &s.SuppressWarning,
		Systems:         &apiSystems,
		WarnCoverage:    &s.WarnCoverage,
	}, nil
}
