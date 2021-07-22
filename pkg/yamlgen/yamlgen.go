// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package yamlgen

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/pkg/errors"
)

// TODO(saswatamcode): Add tests.
// TODO(saswatamcode): Check jennifer code for some safety.
// TODO(saswatamcode): Add mechanism for caching output from generated code.

// getSourceFromMod fetches source code file from $GOPATH/pkg/mod.
func getSourceFromMod(root string, structName string) ([]byte, error) {
	var src []byte
	stopWalk := errors.New("stop walking")

	// Walk source dir.
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// Check if file is Go code.
		if !info.IsDir() && filepath.Ext(path) == ".go" && err == nil {
			src, err = ioutil.ReadFile(path)
			if err != nil {
				return errors.Wrapf(err, "read file for yaml gen %v", path)
			}
			// Check if file has struct.
			if bytes.Contains(src, []byte("type "+structName+" struct")) {
				return stopWalk
			}
		}
		return nil
	})
	if err == stopWalk {
		err = nil
	}

	return src, err
}

// GetSource get source code of file containing the struct.
func GetSource(ctx context.Context, structLocation string) ([]byte, error) {
	// Get struct name.
	loc := strings.Split(structLocation, ":")

	// Check if it is a local file.
	_, err := os.Stat(loc[0])
	if err == nil {
		src, err := ioutil.ReadFile(loc[0])
		if err != nil {
			return nil, errors.Wrapf(err, "read file for yaml gen %v", loc[0])
		}

		// As it is a local file, return source directly.
		return src, nil
	}

	// Not local file so must be module. Will be of form `github.com/bwplotka/mdox@v0.2.2-0.20210712170635-f49414cc6b5a/pkg/mdformatter/linktransformer:Config`.
	// Split using version number (if provided).
	getModule := loc[0]
	moduleName := strings.SplitN(loc[0], "@", 2)
	if len(moduleName) == 2 {
		// Split package dir (if provided).
		pkg := strings.SplitN(moduleName[1], "/", 2)
		if len(pkg) == 2 {
			getModule = moduleName[0] + "@" + pkg[0]
		}
	}
	//TODO(saswatamcode): Handle case where version number not present but package name is.

	// Fetch module.
	cmd := exec.CommandContext(ctx, "go", "get", "-u", getModule)
	err = cmd.Run()
	if err != nil {
		return nil, errors.Wrapf(err, "run %v", cmd)
	}

	// Get GOPATH.
	goPath, ok := os.LookupEnv("GOPATH")
	if !ok {
		return nil, errors.New("GOPATH not set")
	}

	// Get source file of struct.
	file, err := getSourceFromMod(filepath.Join(goPath, "pkg/mod", loc[0]), loc[1])
	if err != nil {
		return nil, err
	}

	return file, nil
}

// GenGoCode generates Go code for yaml gen from structs in src file.
func GenGoCode(src []byte) (string, error) {
	// Create new main file.
	fset := token.NewFileSet()
	generatedCode := jen.NewFile("main")

	// Parse source file.
	f, err := parser.ParseFile(fset, "", src, parser.AllErrors)
	if err != nil {
		return "", err
	}

	// Add imports if needed(will not be used if not required in code).
	for _, s := range f.Imports {
		generatedCode.ImportName(s.Path.Value[1:len(s.Path.Value)-1], "")
	}

	// Init statements for structs.
	var init []jen.Code
	// Declare config map, i.e, `configs := map[string]interface{}{}`.
	init = append(init, jen.Id("configs").Op(":=").Map(jen.String()).Interface().Values())

	// Loop through declarations in file.
	for _, decl := range f.Decls {
		// Cast to generic declaration node.
		if genericDecl, ok := decl.(*ast.GenDecl); ok {
			// Check if declaration spec is `type`.
			if typeDecl, ok := genericDecl.Specs[0].(*ast.TypeSpec); ok {
				var structFields []jen.Code
				// Cast to `type struct`.
				structDecl, ok := typeDecl.Type.(*ast.StructType)
				if !ok {
					generatedCode.Type().Id(typeDecl.Name.Name).Id(string(src[typeDecl.Type.Pos()-1 : typeDecl.Type.End()-1]))
					continue
				}
				fields := structDecl.Fields.List
				arrayInit := make(jen.Dict)

				// Loop and generate fields for each field.
				for _, field := range fields {
					// Each field might have multiple names.
					names := field.Names
					for _, n := range names {
						if n.IsExported() {
							pos := n.Obj.Decl.(*ast.Field)

							// Check if field is a slice type.
							sliceRe := regexp.MustCompile(`.*\[.*\].*`)
							if sliceRe.MatchString(types.ExprString(field.Type)) {
								arrayInit[jen.Id(n.Name)] = jen.Id(string(src[pos.Type.Pos()-1 : pos.Type.End()-1])).Values(jen.Id(string(src[pos.Type.Pos()+1 : pos.Type.End()-1])).Values())
							}

							// Copy struct field to generated code.
							if pos.Tag != nil {
								structFields = append(structFields, jen.Id(n.Name).Id(string(src[pos.Type.Pos()-1:pos.Type.End()-1])).Id(pos.Tag.Value))
							}
						}
					}
				}

				// Add initialize statements for struct via code like `configs["Type"] = Type{}`.
				// If struct has array members, use array initializer via code like `configs["Config"] = Config{ArrayMember: []Type{Type{}}}`.
				init = append(init, jen.Id("configs").Index(jen.Lit(typeDecl.Name.Name)).Op("=").Id(typeDecl.Name.Name).Values(arrayInit))

				// Finally put struct inside generated code.
				generatedCode.Type().Id(typeDecl.Name.Name).Struct(structFields...)
			}
		}
	}

	// Add for loop to iterate through map and return config YAML.
	init = append(init, jen.For(
		jen.List(jen.Id("k"), jen.Id("config")).Op(":=").Range().Id("configs"),
	).Block(
		// We import the cfggen Generate method directly to generate output.
		jen.Qual("fmt", "Println").Call(jen.Lit("---")),
		jen.Qual("fmt", "Println").Call(jen.Id("k")),
		// TODO(saswatamcode): Replace with import from mdox itself once merged.
		// jen.Qual("github.com/bwplotka/mdox/yamlgen", "Generate").Call(jen.Id("config"), jen.Qual("os", "Stderr")),
		jen.Qual("structgen/cfggen", "Generate").Call(jen.Id("config"), jen.Qual("os", "Stderr")),
	))

	// Generate main function in new module.
	generatedCode.Func().Id("main").Params().Block(
		init...,
	)
	return fmt.Sprintf("%#v", generatedCode), nil
}

// execGoCode executes and returns output from generated Go code.
func ExecGoCode(ctx context.Context, mainGo string) ([]byte, error) {
	tmpDir, err := ioutil.TempDir("", "structgen")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// TODO(saswatamcode): Remove once merged.
	// This is weird but need it for getting Generate function inside tmpDir in PR.
	// Once merged this can be removed and can be replaced with just an import in tmpDir/main.go.
	err = os.Mkdir(filepath.Join(tmpDir, "cfggen"), 0700)
	if err != nil {
		return nil, err
	}
	code, err := os.Create(filepath.Join(tmpDir, "cfggen/cfggen.go"))
	if err != nil {
		return nil, err
	}
	defer code.Close()
	_, err = code.Write([]byte(cfggenFile))
	if err != nil {
		return nil, err
	}

	// Copy generated code to main.go.
	main, err := os.Create(filepath.Join(tmpDir, "main.go"))
	if err != nil {
		return nil, err
	}
	defer main.Close()

	_, err = main.Write([]byte(mainGo))
	if err != nil {
		return nil, err
	}

	// Create go.mod in temp dir.
	cmd := exec.CommandContext(ctx, "go", "mod", "init", "structgen")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "run %v", cmd)
	}

	// Import required packages(generate go.sum).
	cmd = exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "run %v", cmd)
	}

	// Execute generate code and return output.
	b := bytes.Buffer{}
	cmd = exec.CommandContext(ctx, "go", "run", "main.go")
	cmd.Dir = tmpDir
	cmd.Stderr = &b
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "run %v out %v", cmd, b.String())
	}

	return b.Bytes(), nil
}

// TODO(saswatamcode): Remove once merged.
// This is weird but need it for getting Generate function inside tmpDir in PR.
// Could also do with commit hash and go.mod, but it would change on each commit in PR).
// Once merged this can be removed and can be replaced with just an import in tmpDir/main.go.
const cfggenFile = `package cfggen

import (
	"io"
	"reflect"

	"github.com/fatih/structtag"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func Generate(obj interface{}, w io.Writer) error {
	// We forbid omitempty option. This is for simplification for doc generation.
	if err := checkForOmitEmptyTagOption(obj); err != nil {
		return errors.Wrap(err, "invalid type")
	}
	return yaml.NewEncoder(w).Encode(obj)
}

func checkForOmitEmptyTagOption(obj interface{}) error {
	return checkForOmitEmptyTagOptionRec(reflect.ValueOf(obj))
}

func checkForOmitEmptyTagOptionRec(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			tags, err := structtag.Parse(string(v.Type().Field(i).Tag))
			if err != nil {
				return errors.Wrapf(err, "%s: failed to parse tag %q", v.Type().Field(i).Name, v.Type().Field(i).Tag)
			}

			tag, err := tags.Get("yaml")
			if err != nil {
				return errors.Wrapf(err, "%s: failed to get tag %q", v.Type().Field(i).Name, v.Type().Field(i).Tag)
			}

			for _, opts := range tag.Options {
				if opts == "omitempty" {
					return errors.Errorf("omitempty is forbidden for config, but spotted on field '%s'", v.Type().Field(i).Name)
				}
			}

			if err := checkForOmitEmptyTagOptionRec(v.Field(i)); err != nil {
				return errors.Wrapf(err, "%s", v.Type().Field(i).Name)
			}
		}

	case reflect.Ptr:
		return errors.New("nil pointers are not allowed in configuration")

	case reflect.Interface:
		return checkForOmitEmptyTagOptionRec(v.Elem())
	}

	return nil
}
`
