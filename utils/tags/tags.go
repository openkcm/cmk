package tags

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.tools.sap/kms/cmk/internal/errs"
)

var ErrBadRepoTag = errors.New("unrecognised repo tag")

// At the minute we define the tags as needed in the source code.
// If we extend for many tags we can introduce a schema and implement
// better validation.

func Get(tag reflect.StructTag, id string) (map[string]string, error) {
	tags := make(map[string]string)

	tagValue := tag.Get(id)
	if tagValue == "" {
		return tags, nil
	}

	for repoTag := range strings.SplitSeq(tagValue, `;`) {
		repoSplitTag := strings.Split(repoTag, `:`)
		switch len(repoSplitTag) {
		case 1:
			tags[repoSplitTag[0]] = ""
		case 2: //nolint:mnd
			tags[repoSplitTag[0]] = repoSplitTag[1]
		default:
			return tags, ErrBadRepoTag
		}
	}

	return tags, nil
}

func CheckBool(tags map[string]string, tagName string, defaultValue bool) (bool, error) {
	tagValue, ok := tags[tagName]
	if !ok {
		return defaultValue, nil
	}

	tagBool, err := strconv.ParseBool(tagValue)
	if err != nil {
		return defaultValue, errs.Wrap(ErrBadRepoTag, err)
	}

	return tagBool, nil
}
