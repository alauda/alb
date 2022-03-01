package utils

import (
	"encoding/json"
	"errors"

	"alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/spf13/cast"
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

// deprecated
// isBracketBalance checks if rawDSL is valid,
// for legacy dsl attribute left and right brackets must be in pairs
func isBracketBalance(rawDSL string) bool {
	var (
		stack  []string
		popVal string
	)
	for _, charCode := range rawDSL {
		if string(charCode) == "(" {
			stack = append(stack, "(")
		} else if string(charCode) == ")" {
			if len(stack) == 0 {
				return false
			}
			popVal, stack = stack[len(stack)-1], stack[:len(stack)-1]
			if popVal != "(" {
				return false
			}
		}
	}
	if len(stack) > 0 {
		return false
	}
	return true
}

// deprecated
func tokenizer(rawDSL string) []string {
	var (
		token          string
		tokens         []string
		nextTokenBegin int
	)
	for idx, charCode := range rawDSL {
		c := string(charCode)
		if c == "(" {
			tokens = append(tokens, "(")
			nextTokenBegin = idx + 1
		} else if c == " " {
			token = rawDSL[nextTokenBegin:idx]
			if token != "" && token != " " && token != ")" {
				tokens = append(tokens, token)
			}
			nextTokenBegin = idx + 1
		} else if c == ")" {
			token = rawDSL[nextTokenBegin:idx]
			if token != "" && token != " " && token != ")" {
				tokens = append(tokens, token)
			}
			tokens = append(tokens, ")")
			nextTokenBegin = idx + 1
		}
	}
	return tokens
}

// deprecated
// parseTokens turns tokens to ast
// note: golang pass-by-copy so we need return param explicitly
func parseTokens(tokens []string) (exp []interface{}, newTokens []string, flag bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrEOF
		}
	}()
	if len(tokens) == 0 {
		return nil, nil, false, ErrEOF
	}
	var token string
	token, tokens = tokens[0], tokens[1:len(tokens)]
	if token != "(" {
		return []interface{}{token}, tokens, true, nil
	}
	for tokens[0] != ")" {
		var (
			t    []interface{}
			flag bool
		)
		t, tokens, flag, err = parseTokens(tokens)
		if err != nil {
			return nil, nil, flag, err
		}
		if flag {
			exp = append(exp, t[0])
		} else {
			exp = append(exp, t)
		}
	}
	tokens = tokens[1:len(tokens)]
	return exp, tokens, false, nil
}

// deprecated
func generateTerm(atomExp []string) v1.DSLXTerm {
	term := v1.DSLXTerm{
		Type: atomExp[1],
	}
	values := []string{
		atomExp[0],
	}
	if KeyTypes[term.Type] {
		term.Key = atomExp[2]
		values = append(values, atomExp[3:len(atomExp)]...)
		term.Values = [][]string{
			values,
		}
	} else {
		values = append(values, atomExp[2:len(atomExp)]...)
		term.Values = [][]string{
			values,
		}
	}
	return term
}

// deprecated
func mergeTerms(terms []v1.DSLXTerm) (v1.DSLXTerm, error) {
	var term v1.DSLXTerm
	if len(terms) == 0 {
		return term, ErrInvalidExp
	}
	term.Key = terms[0].Key
	term.Type = terms[0].Type
	for _, t := range terms {
		term.Values = append(term.Values, t.Values[0])
	}
	return term, nil
}

// deprecated
func generateDSLX(exp []interface{}) (v1.DSLX, error) {
	var dslx v1.DSLX
	if len(exp) == 0 {
		return dslx, nil
	}
	if exp[0] == OP_AND {
		// "[AND [EQ HOST baidu.com] [STARTS_WITH URL /lorem]]"
		exp = exp[1:len(exp)]
	} else {
		// "[EQ HOST baidu.com]"
		exp = []interface{}{exp}
	}
	for _, subExp := range exp {
		testSubExp, err := cast.ToSliceE(subExp)
		if err != nil || len(testSubExp) == 0 {
			return nil, ErrInvalidExp
		}
		currentOP, err := cast.ToStringE(testSubExp[0])
		if err != nil {
			return nil, err
		}
		if LogicalMatcher[currentOP] {
			//[OR [STARTS_WITH URL /kubernetes/] [STARTS_WITH URL /k8s/]]
			if currentOP != OP_OR {
				return nil, ErrUnsupportedExp
			}
			rest := testSubExp[1:]
			var terms []v1.DSLXTerm
			for _, i := range rest {
				trueSubExp, err := cast.ToStringSliceE(i)
				if err != nil {
					return nil, err
				}
				for _, e := range trueSubExp {
					if e == "" {
						return nil, ErrInvalidExp
					}
				}
				term := generateTerm(trueSubExp)
				terms = append(terms, term)
			}
			term, err := mergeTerms(terms)
			if err != nil {
				return nil, err
			}
			dslx = append(dslx, term)
		} else {
			//[EQ COOKIE test 12]
			trueSubExp, err := cast.ToStringSliceE(testSubExp)
			if err != nil {
				return nil, err
			}
			for _, e := range trueSubExp {
				if e == "" {
					return nil, ErrInvalidExp
				}
			}
			term := generateTerm(trueSubExp)
			dslx = append(dslx, term)
		}
	}
	return dslx, nil
}

// deprecated
func DSL2DSLX(rawDSL string) (v1.DSLX, error) {
	if !isBracketBalance(rawDSL) {
		return nil, errors.New("invalid dsl")
	}
	var dslx []v1.DSLXTerm
	tokens := tokenizer(rawDSL)
	ast, _, _, err := parseTokens(tokens)
	if err != nil {
		return nil, err
	}
	// 前端生成的dsl，最多有两层
	if len(ast) == 0 {
		return dslx, nil
	}
	dslx, err = generateDSLX(ast)
	if err != nil {
		return dslx, err
	}
	return dslx, nil
}

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
				var term = []string{val[0], dsl.Type}
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
