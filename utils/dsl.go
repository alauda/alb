package utils

import (
	"encoding/json"
	"errors"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
)

var (
	KeyTypes = map[string]bool{
		KEY_COOKIE: true,
		KEY_HEADER: true,
		KEY_PARAM:  true,
	}
	MatcherWithParam = map[string]bool{
		OP_EQ:          true,
		OP_IN:          true,
		OP_RANGE:       true,
		OP_REGEX:       true,
		OP_STARTS_WITH: true,
		OP_ENDS_WITH:   true,
	}
	LogicalMatcher = map[string]bool{
		OP_AND: true,
		OP_OR:  true,
	}
)

var (
	ErrEOF            = errors.New("unexpected EOF while parsing")
	ErrInvalidExp     = errors.New("invalid exp")
	ErrUnsupportedExp = errors.New("unsupported exp")
)

func DSLX2Internal(dslx v1.DSLX) ([]interface{}, error) {
	var internal []interface{}
	if len(dslx) == 0 {
		return internal, nil
	}
	if len(dslx) > 1 {
		internal = append(internal, OP_AND)
	}
	for _, dsl := range dslx {
		var tmp []interface{}
		if len(dsl.Values) == 0 {
			continue
		}
		if len(dsl.Values) > 1 {
			tmp = append(tmp, OP_OR)
			for _, val := range dsl.Values {
				if len(val) < 2 {
					return nil, errors.New("invalid dslx values")
				}
				term := []string{val[0], dsl.Type}
				rest := val[1:]
				if dsl.Key != "" {
					term = append(term, dsl.Key)
				}
				term = append(term, rest...)
				tmp = append(tmp, term)
			}
			internal = append(internal, tmp)
		} else {
			rest := dsl.Values[0][1:len(dsl.Values[0])]
			term := []string{dsl.Values[0][0], dsl.Type}
			if dsl.Key != "" {
				term = append(term, dsl.Key)
			}
			term = append(term, rest...)
			internal = append(internal, term)
		}
	}
	return internal, nil
}

func InternalDSLLen(dslx []interface{}) int {
	jsonStr, err := json.Marshal(dslx)
	if err != nil {
		return 0
	}
	return len(jsonStr)
}
