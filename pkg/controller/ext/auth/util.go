package auth

import (
	"strings"

	. "alauda.io/alb2/pkg/controller/ext/auth/types"
)

// NOTE: 因为实现原理的不同，我们的parse实际上要比ingress-nginx的更宽松。。$$ 在ingress-nginx会报错
// 这里我们假设不会有这种异常的annotation
func ParseVarString(s string) (VarString, error) {
	ret := VarString{}
	if strings.TrimSpace(s) == "" {
		return ret, nil
	}
	if !strings.Contains(s, "$") {
		ret = append(ret, s)
		return ret, nil
	}
	buff := ""
	cur := 0
	take := func() string {
		cur++
		return string(s[cur-1])
	}
	peek := func() (string, bool) {
		if cur == len(s) {
			return "", true
		}
		return string(s[cur]), false
	}
	for {
		c, eof := peek()
		if eof {
			break
		}
		if c == "$" {
			if buff != "" {
				ret = append(ret, buff)
			}
			buff = take()
			continue
		}
		// 变量只能是字母数字下划线
		is_var := (c >= "0" && c <= "9") || (c >= "a" && c <= "z") || (c >= "A" && c <= "Z") || c == "_"
		if !is_var {
			// 不是变量模式,继续
			if len(buff) > 0 && buff[0] != '$' {
				buff += take()
				continue
			}
			// 是变量模式
			if len(buff) > 0 && buff[0] == '$' {
				if c == "{" {
					buff += take()
					continue
				}
				// ${} 模式
				if c == "}" && len(buff) >= 2 && buff[1] == '{' {
					_ = take()
					buff = "$" + buff[2:]
					ret = append(ret, buff)
					buff = ""
					continue
				}
				// 其他字符都会导致立刻退出变量模式
				ret = append(ret, buff)
				buff = take()
				continue
			}
		}
		buff += take()
	}
	// 处理最后一个
	if buff != "" {
		ret = append(ret, buff)
	}
	return ret, nil
}
