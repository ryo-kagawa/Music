package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ryo-kagawa/Music/types/cue"
	"github.com/ryo-kagawa/go-utils/arrays"
	"github.com/ryo-kagawa/go-utils/commandline"
)

type Command struct{}

var _ = (commandline.RootCommand)(Command{})

func (Command) Execute(arguments []string) (string, error) {
	cuePath := arguments[0]
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	cueContents, err := cue.Load(cuePath)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(
		config.FlacExePath,
		append(
			[]string{
				"--force",
				"--warnings-as-errors",
				"--verify",
				"--replay-gain",
				"--compression-level-8",
				"--no-padding",
			},
			arrays.Map(
				cueContents.Files,
				func(file cue.File) string {
					return filepath.Join(filepath.Dir(cuePath), file.Name)
				},
			)...,
		)...,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	cueContents.Files = arrays.Map(
		cueContents.Files,
		func(file cue.File) cue.File {
			if filepath.Ext(file.Name) != ".flac" {
				os.Remove(filepath.Join(filepath.Dir(cuePath), file.Name))
				file.Name = strings.TrimSuffix(file.Name, filepath.Ext(file.Name)) + ".flac"
			}
			return file
		},
	)
	if err := cueContents.OutputCuefile(cuePath); err != nil {
		return "", err
	}

	return "", nil
}
