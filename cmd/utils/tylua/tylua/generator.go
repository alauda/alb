package tylua

import (
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Generator for one or more input packages, responsible for linking
// them together if necessary.
type TyLua struct {
	conf *Config

	packageGenerators map[string]*PackageGenerator
}

// Responsible for generating the code for an input package
type PackageGenerator struct {
	conf         *PackageConfig
	pkg          *packages.Package
	GoFiles      []string
	types        map[string]string
	pendingTypes map[string]bool
	typesOrder   map[string]int
}

func New(config *Config) *TyLua {
	return &TyLua{
		conf:              config,
		packageGenerators: make(map[string]*PackageGenerator),
	}
}

func (g *TyLua) SetTypeMapping(goType string, tsType string) {
	for _, p := range g.conf.Packages {
		p.TypeMappings[goType] = tsType
	}
}

func (g *TyLua) Generate(pkg_names []string, boot string, out string) error {
	log.Printf("Generating for packages: %v", pkg_names)
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedSyntax | packages.NeedFiles,
	}, pkg_names...)
	if err != nil {
		return err
	}
	know_types := map[string]string{}
	types_order := map[string]int{}
	pending_types := map[string]bool{boot: true}
	for i := 0; i < 10; i++ {
		allok := true
		pd := ""
		for k, v := range pending_types {
			if v == true {
				pd = pd + " " + k
				allok = false
			}
		}
		if allok {
			break
		}
		log.Printf("pending %v", pd)
		for i, pkg := range pkgs {
			if len(pkg.GoFiles) == 0 {
				log.Printf("no input go files for package index %d", i)
				continue
			}
			if len(pkg.Errors) > 0 {
				return fmt.Errorf("err %+v", pkg.Errors)
			}

			// log.Printf("pkg x %v", pkg.Name)
			pkgConfig := &PackageConfig{
				TypeMappings: map[string]string{
					"albv1.PortNumber": "number",
				},
			}
			pkgGen := &PackageGenerator{
				conf:    pkgConfig,
				GoFiles: pkg.GoFiles,

				pkg:          pkg,
				types:        know_types,
				pendingTypes: pending_types,
				typesOrder:   types_order,
			}
			g.packageGenerators[pkg.PkgPath] = pkgGen
			pkgGen.Resovle()
		}
	}
	codes := make([]string, len(know_types)+1)
	for k, v := range know_types {
		codes[types_order[k]] = v
	}
	log.Printf("resolve ok %v", out)
	return os.WriteFile(out, []byte(strings.Join(codes, "")), os.FileMode(0o644))
}
