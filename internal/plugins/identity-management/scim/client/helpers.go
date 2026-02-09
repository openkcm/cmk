package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/common-sdk/pkg/pointers"

)

var (
	ErrNoFilter     = errors.New("filter not provided")
	ErrMarshallFail = errors.New("failed to marshal search request")
)

func buildBodyFromParams(filter FilterExpression, count *int, cursor *string) (io.Reader, error) {
	searchRequest := SearchRequest{
		Schemas: []string{SearchRequestSchema},
		Count:   count,
		Cursor:  cursor,
	}

	if filter == nil || (filter == NullFilterExpression{}) {
		return nil, ErrNoFilter
	}

	searchRequest.Filter = pointers.To(filter.ToString())

	jsonBody, err := json.Marshal(searchRequest)
	if err != nil {
		return nil, errs.Wrap(ErrMarshallFail, err)
	}

	return bytes.NewReader(jsonBody), nil
}

func buildQueryStringFromParams(filter FilterExpression, cursor *string, count *int) string {
	query := url.Values{}
	if cursor != nil {
		query.Add("cursor", *cursor)
	}

	if count != nil {
		query.Add("count", strconv.Itoa(*count))
	}

	if (filter != nil) && (filter != NullFilterExpression{}) {
		query.Add("filter", (filter).ToString())
	}

	return query.Encode()
}
