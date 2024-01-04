package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/spf13/cast"

	"alauda.io/alb2/driver"
	. "alauda.io/alb2/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	Name      string
	Namespace string
	Domain    string
	dryRun    = flag.Bool("dry-run", true, "dry run flag")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()
	ensureEnv()
	k8sDriver, err := driver.GetDriver(context.TODO())
	if err != nil {
		panic(err)
	}
	allRules, err := k8sDriver.ALBClient.CrdV1().Rules(Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("alb2.%s/name=%s", Domain, Name),
	})
	if err != nil {
		panic(err)
	}
	for _, rl := range allRules.Items {
		if rl.Spec.DSLX != nil {
			continue
		}
		dslx, err := DSL2DSLX(rl.Spec.DSL)
		if err != nil {
			klog.Warningf("failed to convert rule %s/%s dsl %s to dslx", rl.Namespace, rl.Name, rl.Spec.DSL)
		}
		rl.Spec.DSLX = dslx
		klog.Infof("convert rule %s/%s dsl: %s to dslx: %+v", rl.Namespace, rl.Name, rl.Spec.DSL, dslx)
		if !*dryRun {
			if _, err = k8sDriver.ALBClient.CrdV1().Rules(Namespace).Update(context.TODO(), &rl, metav1.UpdateOptions{}); err != nil {
				klog.Error(err)
			}
		}
	}
}

func ensureEnv() {
	Name = os.Getenv("NAME")
	Namespace = os.Getenv("NAMESPACE")
	Domain = os.Getenv("DOMAIN")
	klog.Info("NAME: ", Name)
	klog.Info("NAMESPACE: ", Namespace)
	klog.Info("DOMAIN: ", Domain)
	if strings.TrimSpace(Name) == "" &&
		strings.TrimSpace(Namespace) == "" &&
		strings.TrimSpace(Domain) == "" {
		panic("you must set NAME and NAMESPACE and DOMAIN env")
	}
}

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
	return len(stack) == 0
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
		switch c {
		case "(":
			tokens = append(tokens, "(")
			nextTokenBegin = idx + 1
		case " ":
			token = rawDSL[nextTokenBegin:idx]
			if token != "" && token != " " && token != ")" {
				tokens = append(tokens, token)
			}
			nextTokenBegin = idx + 1
		case ")":
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
	token, tokens = tokens[0], tokens[1:]
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
	tokens = tokens[1:]
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
		values = append(values, atomExp[3:]...)
		term.Values = [][]string{
			values,
		}
	} else {
		values = append(values, atomExp[2:]...)
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

func generateDSLX(exp []interface{}) (v1.DSLX, error) {
	var dslx v1.DSLX
	if len(exp) == 0 {
		return dslx, nil
	}
	if exp[0] == OP_AND {
		// "[AND [EQ HOST baidu.com] [STARTS_WITH URL /lorem]]"
		exp = exp[1:]
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
			// [OR [STARTS_WITH URL /kubernetes/] [STARTS_WITH URL /k8s/]]
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
			// [EQ COOKIE test 12]
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
