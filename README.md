# mdox

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/bwplotka/mdox) [![Latest Release](https://img.shields.io/github/release/bwplotka/mdox.svg?style=flat-square)](https://github.com/bwplotka/mdox/releases/latest) [![CI](https://github.com/bwplotka/mdox/workflows/go/badge.svg)](https://github.com/bwplotka/mdox/actions?query=workflow%3Ago) [![Go Report Card](https://goreportcard.com/badge/github.com/bwplotka/mdox)](https://goreportcard.com/report/github.com/bwplotka/mdox)

`mdox` (spelled as `md docs`) is a CLI for maintaining automated, high quality project documentation and website leveraging [Github Flavored Markdown](https://github.github.com/gfm/) and git.

This project can be used both as CLI as well as library.

## Goals

Allow projects to have self-updating up-to-date documentation available in both markdown (e.g readable from GitHub) and static HTML. Hosted in the same repository as code and integrated with Pull Requests CI, hosting CD and code generation.

## Features

* Enhanced and consistent formatting for markdown files in [GFM](https://github.github.com/gfm/) format, focused on readability.
* Auto generation of code block content based on `mdox-exec` directives (see [#code-generation](#code-generation)). Useful for:
  * Generating help output from CLI --help
  * Generating example YAML from Go configuration struct (+comments)
* Robust and fast relative and remote link checking.
* Website integration:
  * "Localizing" links to relative docs if specified (useful for multi-domain websites or multi-version doc).
    * This allows smooth integration with static document websites like [Docusaurus](https://docusaurus.io/) or [hugo](https://gohugo.io) based themes!
  * Flexible pre-processing allowing easy to use GitHub experience as well as website.

## Usage

Just run `mdox fmt` and pass markdown files (or glob matching those).

For example this README is formatted by the CI on every PR using [`mdox fmt -l *.md` command](https://github.com/bwplotka/mdox/blob/9e183714070f464b1ef089da3df8048aff1abeda/Makefile#L59).

```bash mdox-exec="mdox fmt --help"
usage: mdox fmt [<flags>] <files>...

Formats in-place given markdown files uniformly following GFM (Github Flavored
Markdown: https://github.github.com/gfm/). Example: mdox fmt *.md

Flags:
  -h, --help                     Show context-sensitive help (also try
                                 --help-long and --help-man).
      --version                  Show application version.
      --log.level=info           Log filtering level.
      --log.format=clilog        Log format to use.
      --check                    If true, fmt will not modify the given files,
                                 instead it will fail if files needs formatting
      --code.disable-directives  If false, fmt will parse custom fenced code
                                 directives prefixed with 'mdox-gen' to
                                 autogenerate code snippets. For example:
                                 
                                   ```<lang> mdox-exec="<executable + arguments>"
                                 
                                 This directive runs executable with arguments
                                 and put its stderr and stdout output inside
                                 code block content, replacing existing one.
      --anchor-dir=ANCHOR-DIR    Anchor directory for all transformers. PWD is
                                 used if flag is not specified.
      --links.localize.address-regex=LINKS.LOCALIZE.ADDRESS-REGEX  
                                 If specified, all HTTP(s) links that target a
                                 domain and path matching given regexp will be
                                 transformed to relative to anchor dir path (if
                                 exists).Absolute path links will be converted
                                 to relative links to anchor dir as well.
  -l, --links.validate           If true, all links will be validated
      --links.validate.config-file=<file-path>  
                                 Path to YAML file for skipping link check, with
                                 spec defined in
                                 github.com/bwplotka/mdox/pkg/linktransformer.ValidatorConfig
      --links.validate.config=<content>  
                                 Alternative to 'links.validate.config-file'
                                 flag (mutually exclusive). Content of YAML file
                                 for skipping link check, with spec defined in
                                 github.com/bwplotka/mdox/pkg/linktransformer.ValidatorConfig

Args:
  <files>  Markdown file(s) to process.

```

### Code Generation

It's not uncommon that documentation is explaining code or configuration snippets. One of the challenges of such documentation is keeping it up to date. This is where `mdox` code block directives comes handy! To ensure mdox will auto update code snippet add `mdox-exec="<whatever command you want take output from>"` after language directive on code block.

For example this Readme contains `mdox --help` which is has to be auto generated on every PR:

```markdown
``` bash mdox-exec="mdox fmt --help"
...
```

This also enables auto updating snippets of code in code blocks using tools like `sed`. For example, below code block directive will auto update and insert lines 3 to 6 from main.go into code block.

```markdown
```go mdox-exec="sed -n '3,6p' main.go"
...
```

You can disable this feature by specifying `--code.disable-directives`

### Installing

Requirements to build this tool:

* Go 1.15+
* Linux or MacOS

```shell
go get github.com/bwplotka/mdox && go mod tidy
```

or via [bingo](https://github.com/bwplotka/bingo) if want to pin it:

```shell
bingo get -u github.com/bwplotka/mdox
```

### Production Usage

* [Thanos](https://github.com/bwplotka/thanos) (TBD)

## Contributing

Any contributions are welcome! Just use GitHub Issues and Pull Requests as usual. We follow [Thanos Go coding style](https://thanos.io/tip/contributing/coding-style-guide.md/) guide.

## Initial Author

[@bwplotka](https://bwplotka.dev)
