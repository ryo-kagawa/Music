package main

import (
	"os"
	"path/filepath"

	"github.com/ryo-kagawa/Music/types/cue"
	"github.com/ryo-kagawa/go-utils/commandline"
)

type Command struct{}

var _ = (commandline.RootCommand)(Command{})

func (Command) Execute(arguments []string) (string, error) {
	cuePath := arguments[0]
	outputDirectory := filepath.Join(filepath.Dir(cuePath), "track")
	cue, err := cue.Load(cuePath)
	if err != nil {
		return "", err
	}
	cue = cue.SplitTrack()
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return "", err
	}
	if err := cue.OutputWave(outputDirectory); err != nil {
		return "", err
	}
	if err := cue.OutputCuefile(outputDirectory); err != nil {
		return "", err
	}

	return "", nil
}
