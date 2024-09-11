package test_utils

import (
	"strconv"
	"strings"

	. "alauda.io/alb2/utils/test_utils"
	. "github.com/onsi/gomega"
	gngc "github.com/tufanbarisyildirim/gonginx/config"
	gngd "github.com/tufanbarisyildirim/gonginx/dumper"
	gngp "github.com/tufanbarisyildirim/gonginx/parser"
)

func AssertNgxBlockEq(left gngc.IBlock, right string) {
	left_blk_str := gngd.DumpBlock(left, gngd.IndentedStyle)
	b_blk_str := "x {" + right + "}"
	c, err := gngp.NewStringParser(b_blk_str, gngp.WithSkipValidDirectivesErr()).Parse()
	GinkgoNoErr(err)
	blk := c.FindDirectives("x")[0].GetBlock()
	right_blk_str := gngd.DumpBlock(blk, gngd.IndentedStyle)
	Expect(left_blk_str).To(Equal(right_blk_str))
}

func DumpNgxBlockEq(blk gngc.IBlock) string {
	return gngd.DumpBlock(blk, gngd.IndentedStyle)
}

func FindNestDirectivesRaw(ngxconf string, root string, directiveName string) gngc.IDirective {
	p, err := gngp.NewStringParser(ngxconf, gngp.WithSkipValidDirectivesErr()).Parse()
	if err != nil {
		Panic()
	}
	return FindNestDirectives(p, root, directiveName)
}

func FindNamedHttpLocationRaw(ngxconf string, port string, locname string) *gngc.Location {
	p, err := gngp.NewStringParser(ngxconf, gngp.WithSkipValidDirectivesErr()).Parse()
	if err != nil {
		Panic()
	}
	// FindNestDirectives(p, "http",".server")
	for _, d := range p.FindDirectives("server") {
		lss := d.GetBlock().FindDirectives("listen")
		if len(lss) == 0 {
			continue
		}
		find := false
		for _, l := range lss {
			if l.GetParameters()[0] == "0.0.0.0"+":"+port {
				find = true
			}
		}
		if !find {
			continue
		}
		locs := d.GetBlock().FindDirectives("location")
		if len(lss) == 0 {
			continue
		}
		for _, l := range locs {
			if l.GetParameters()[0] == locname {
				loc, _ := l.(*gngc.Location)
				// fmt.Println("loc", l.GetParameters()[0], "locname", locname, "is", is)
				return loc
			}
		}
	}
	return nil
}

func FindNestDirectives(p *gngc.Config, root string, directiveName string) gngc.IDirective {
	// directiveName is like "http.server[1].location.xx"

	parts := strings.Split(directiveName, ".")
	current := p.FindDirectives(root)[0].GetBlock()

	for i, part := range parts {
		if part == "" {
			continue
		}

		if !strings.Contains(part, "[") {
			current = current.FindDirectives(part)[0].GetBlock()
			continue
		}
		// Check if the part contains an index
		if idx := strings.Index(part, "["); idx != -1 {
			name := part[:idx]
			index, err := strconv.Atoi(part[idx+1 : len(part)-1])
			if err != nil {
				return nil
			}

			directives := current.FindDirectives(name)
			if index >= len(directives) {
				return nil
			}
			if i == len(parts)-1 {
				return directives[index]
			}
			current = directives[index].GetBlock()
		}
	}
	return nil
}
