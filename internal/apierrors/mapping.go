package apierrors

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/utils/ptr"
)

type APIErrorMapper struct {
	APIErrors      []APIErrors
	PriorityErrors []APIErrors
}

type APIErrors struct {
	Errors        []error
	ExposedError  cmkapi.DetailedError
	ContextGetter func(error) map[string]any
}

var apiErrorMapper = APIErrorMapper{
	APIErrors: slices.Concat(
		keyConfiguration,
		keyVersion,
		workflow,
		system,
		label,
		tags,
		key,
		tenantconfig,
		groups,
		tenants,
		defaultMapper,
	),
	PriorityErrors: highPrio,
}

func TransformToAPIError(ctx context.Context, err error) *cmkapi.ErrorMessage {
	e := apiErrorMapper.transform(ctx, err)
	if e == nil {
		log.Info(ctx, "No appropriate error mapping. Defaulting to generic 500",
			slog.String(slogctx.ErrKey, err.Error()))

		e = ptr.PointTo(InternalServerErrorMessage())
	}

	return e
}

// The rules to find the best match is as follows:
// 1. If error is in priority APIErrorMapping return the priority one
// 1. Return APIError containing the highest number of errors in err chain
func (m *APIErrorMapper) transform(ctx context.Context, err error) *cmkapi.ErrorMessage {
	e, ok := m.containsAsPriority(err)
	if ok {
		return e
	}

	result := m.getBestMatches(err)

	debugMappingCandidates(ctx, err, result)

	if len(result) == 0 {
		return nil
	}

	selected := result[0]

	detail := selected.ExposedError
	if selected.ContextGetter != nil {
		detail.Context = ptr.PointTo(selected.ContextGetter(err))
	}

	return &cmkapi.ErrorMessage{Error: detail}
}

// Checks if the err is in the priorityErrors group
// If true, return the ErrorMap
func (m *APIErrorMapper) containsAsPriority(err error) (*cmkapi.ErrorMessage, bool) {
	for _, priorityErrors := range m.PriorityErrors {
		if countMatchingErrors(err, priorityErrors.Errors) > 0 {
			return &cmkapi.ErrorMessage{Error: priorityErrors.ExposedError}, true
		}
	}

	return nil, false
}

// Gets the APIErrors with the highest amount of matching errors as the err target chain
func (m *APIErrorMapper) getBestMatches(err error) []APIErrors {
	minCount := 1

	var result []APIErrors

	for _, apiErr := range m.APIErrors {
		count := countMatchingErrors(err, apiErr.Errors)

		// Skip if APIError contains errors that are not in the err
		if len(apiErr.Errors) > count {
			continue
		}

		if count == minCount {
			result = append(result, apiErr)
		} else if count > minCount {
			minCount = count
			result = []APIErrors{apiErr}
		}
	}

	return result
}

// countMatchingErrors counts the number of errors in candidates that match err
func countMatchingErrors(err error, candidates []error) int {
	matchCount := 0

	for _, candidateErr := range candidates {
		if errors.Is(err, candidateErr) {
			matchCount++
		}
	}

	return matchCount
}

// debugMappingCandidates logs the mapping candidates for debugging purposes
func debugMappingCandidates(ctx context.Context, err error, mappingCandidates []APIErrors) {
	if len(mappingCandidates) > 1 {
		log.Debug(ctx, "Mapping more than one error; selecting candidates",
			slog.String(slogctx.ErrKey, err.Error()))

		for position, me := range mappingCandidates {
			log.Debug(ctx, "Matched candidate",
				slog.Int("position", position),
				slog.String("code", me.ExposedError.Code),
				slog.String("status", http.StatusText(me.ExposedError.Status)),
				slog.Int("matchedLength", len(me.Errors)),
			)
		}
	}
}
