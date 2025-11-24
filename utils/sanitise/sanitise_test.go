package sanitise_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/utils/sanitise"
)

// The reflect sets these in-situ, even when not pointers so we need to reset
// for each test
const strXSS1 = "<SCRIPT></SCRIPT>"
const strSAN1 = ""

const strXSS2 = "Hello <SCRIPT></SCRIPT> Bye"
const strSAN2 = "Hello  Bye"

const strXSS3 = "Bye <SCRIPT></SCRIPT> Hello"
const strSAN3 = "Bye  Hello"

func TestSanitisation(t *testing.T) {
	t.Run("Should sanitise strings", func(t *testing.T) {
		input := strXSS1
		output := strSAN1
		ret, err := sanitise.Stringlikes(&input)
		assert.NoError(t, err)
		assert.Equal(t, output, *ret)
	})

	t.Run("Should sanitise string lists", func(t *testing.T) {
		testStrXSS1 := strXSS1
		testStrXSS2 := strXSS2

		input := []string{testStrXSS1, testStrXSS2}
		output := []string{strSAN1, strSAN2}
		ret, err := sanitise.Stringlikes(&input)
		assert.NoError(t, err)
		assert.Equal(t, output, *ret)
	})

	t.Run("Should sanitise string pointer lists", func(t *testing.T) {
		testStrXSS1 := strXSS1
		testStrSAN1 := strSAN1

		testStrXSS2 := strXSS2
		testStrSAN2 := strSAN2

		input := []*string{&testStrXSS1, &testStrXSS2}
		output := []*string{&testStrSAN1, &testStrSAN2}
		ret, err := sanitise.Stringlikes(&input)
		assert.NoError(t, err)
		assert.Equal(t, output, *ret)
	})

	t.Run("Should sanitise maps", func(t *testing.T) {
		map1 := map[string]string{"Key<SCRIPT></SCRIPT>": "Value<SCRIPT></SCRIPT>"}

		type ss struct {
			M map[string]string
		}

		input := ss{M: map1}
		map2 := map[string]string{"Key": "Value"}
		output := ss{M: map2}

		ret, err := sanitise.Stringlikes(&input)
		assert.NoError(t, err)
		assert.Equal(t, &output, ret)
	})

	t.Run("Should sanitise structs", func(t *testing.T) {
		testStrXSS1 := strXSS1
		testStrSAN1 := strSAN1

		testStrXSS2 := strXSS2
		testStrSAN2 := strSAN2

		testStrXSS3 := strXSS3
		testPtrStrXSS3 := &testStrXSS3
		testStrSAN3 := strSAN3
		testPtrStrSAN3 := &testStrSAN3

		type s1 struct {
			I   int
			S   string
			Ps  *string
			Pps **string
		}

		type s2 struct {
			S1   s1
			S1s  []s1
			Ps1s *[]s1
			S1ps []*s1
			Ps1  *s1
			Pps1 **s1
		}

		s1inst1 := s1{I: 10, S: testStrXSS1, Ps: &testStrXSS2, Pps: &testPtrStrXSS3}
		s1inst2 := s1{I: 11, S: testStrXSS1, Ps: &testStrXSS2, Pps: &testPtrStrXSS3}
		ps1inst1 := &s1inst1
		s1Slice := []s1{s1inst1, s1inst2}
		input := s2{S1: s1inst1, S1s: s1Slice, Ps1s: &s1Slice,
			S1ps: []*s1{&s1inst1, &s1inst2}, Ps1: &s1inst1, Pps1: &ps1inst1}

		s1inst1Ex := s1{I: 10, S: testStrSAN1, Ps: &testStrSAN2, Pps: &testPtrStrSAN3}
		s1inst2Ex := s1{I: 11, S: testStrSAN1, Ps: &testStrSAN2, Pps: &testPtrStrSAN3}
		ps1inst1Ex := &s1inst1Ex
		s1SliceEx := []s1{s1inst1Ex, s1inst2Ex}
		output := s2{S1: s1inst1Ex, S1s: s1SliceEx, Ps1s: &s1SliceEx,
			S1ps: []*s1{&s1inst1Ex, &s1inst2Ex}, Ps1: &s1inst1Ex, Pps1: &ps1inst1Ex}

		ret, err := sanitise.Stringlikes(&input)
		assert.NoError(t, err)
		assert.Equal(t, output, *ret)

		// Just something more explicit for sanity:
		assert.Equal(t, "Bye  Hello", **(*output.Pps1).Pps)
	})

	t.Run("Should sanitise json", func(t *testing.T) {
		testStrXSS := strXSS2

		var input json.RawMessage = []byte(testStrXSS)

		var output json.RawMessage = []byte(`Hello &lt;SCRIPT&gt;&lt;/SCRIPT&gt; Bye`)

		ret, err := sanitise.Stringlikes(&input)
		assert.NoError(t, err)
		assert.Equal(t, output, *ret)
	})
}

func TestSanitiseTurnedOff(t *testing.T) {
	type s struct {
		I int
		S string `repo:"sanitise:false"`
	}

	testStrXSS := strXSS2
	sinst := s{I: 10, S: testStrXSS}
	sinstEx := s{I: 10, S: testStrXSS}
	ret, err := sanitise.Stringlikes(&sinst)
	assert.NoError(t, err)
	assert.Equal(t, sinstEx, *ret)
}

func TestImportantValuesNotSanitised(t *testing.T) {
	input1 := "10d90855-cf4a-4396-8db7-caf41171766f"
	output1 := "10d90855-cf4a-4396-8db7-caf41171766f"
	ret1, err := sanitise.Stringlikes(&input1)
	assert.NoError(t, err)
	assert.Equal(t, output1, *ret1)
}

func TestNonSupportedTypes(t *testing.T) {
	_, err := sanitise.Stringlikes(map[int]int{1: 2})
	assert.ErrorIs(t, err, sanitise.ErrUnsupportedSanitisationType)
}
