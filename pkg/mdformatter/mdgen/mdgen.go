// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdgen

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/bwplotka/mdox/pkg/yamlgen"
	"github.com/mattn/go-shellwords"

	"github.com/pkg/errors"
)

const (
	infoStringKeyExec     = "mdox-exec"
	infoStringKeyExitCode = "mdox-expect-exit-code"
	infoStringKeyGoStruct = "mdox-gen-go-struct"
)

type genCodeBlockTransformer struct{}

func NewCodeBlockTransformer() *genCodeBlockTransformer {
	return &genCodeBlockTransformer{}
}

func (t *genCodeBlockTransformer) TransformCodeBlock(ctx mdformatter.SourceContext, infoString []byte, code []byte) ([]byte, error) {
	if len(infoString) == 0 {
		return code, nil
	}

	infoFiels, err := shellwords.NewParser().Parse(string(infoString))
	if err != nil {
		return nil, errors.Wrapf(err, "parsing info string %v", string(infoString))
	}
	infoStringAttr := map[string]string{}
	for i, field := range infoFiels {
		val := []string{field}
		if i := strings.Index(field, "="); i != -1 {
			val = []string{field[:i], field[i+1:]}
		}
		if i == 0 && len(val) == 2 {
			return nil, errors.Errorf("missing language info in fenced code block. Got info string %q", string(infoString))
		}
		switch val[0] {
		case infoStringKeyExec:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %s=\"<value1>\" but got %s", val[0], infoStringKeyExec, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		case infoStringKeyExitCode:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %s=\"<value1>\" but got %s", val[0], infoStringKeyExitCode, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		case infoStringKeyGoStruct:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %s=\"<value1>\" but got %s", val[0], infoStringKeyGoStruct, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		}
	}

	if len(infoStringAttr) == 0 {
		// Code fence without mdox attributes.
		return code, nil
	}

	if execCmd, ok := infoStringAttr[infoStringKeyExec]; ok {
		if len(infoStringAttr) > 2 {
			return nil, errors.Errorf("got ambiguous attributes: %v. Expected format for %q is e.g ```text %q=<value> . Got info string %q", infoStringAttr, infoStringKeyExec, infoStringKeyExec, string(infoString))
		}
		execArgs, err := shellwords.NewParser().Parse(execCmd)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing exec command %v", execCmd)
		}

		// Execute and render output.
		b := bytes.Buffer{}
		cmd := exec.CommandContext(ctx, execArgs[0], execArgs[1:]...)
		cmd.Stderr = &b
		cmd.Stdout = &b
		if err := cmd.Run(); err != nil {
			expectedCode, _ := strconv.Atoi(infoStringAttr[infoStringKeyExitCode])
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == expectedCode {
				return b.Bytes(), nil
			}
			return nil, errors.Wrapf(err, "run %v, out: %v", execCmd, b.String())
		}
		return b.Bytes(), nil
	}

	if fileWithStruct, ok := infoStringAttr[infoStringKeyGoStruct]; ok {
		// This is like mdox-gen-go-struct=<filename>:structname for now.
		fs := strings.Split(fileWithStruct, ":")
		src, err := ioutil.ReadFile(fs[0])
		if err != nil {
			return nil, errors.Wrapf(err, "read file for yaml gen %v", fs[0])
		}

		generatedCode, err := yamlgen.GenGoCode(src)
		if err != nil {
			return nil, errors.Wrapf(err, "generate code for yaml gen %v", fs[0])
		}

		b, err := yamlgen.ExecGoCode(ctx, generatedCode)
		if err != nil {
			return nil, errors.Wrapf(err, "execute generated code for yaml gen %v", fs[0])
		}

		// TODO(saswatamcode): This feels sort of hacky, need better way of printing.
		// Remove `---` and check struct name.
		yamls := bytes.Split(b, []byte("---"))
		for _, yaml := range yamls {
			lines := bytes.Split(yaml, []byte("\n"))
			if len(lines) > 1 {
				if string(lines[1]) == fs[1] {
					ret := bytes.Join(lines[2:len(lines)-1], []byte("\n"))
					ret = append(ret, []byte("\n")...)
					return ret, nil
				}
			}
		}
	}

	panic("should never get here")
}

func (t *genCodeBlockTransformer) Close(ctx mdformatter.SourceContext) error { return nil }
