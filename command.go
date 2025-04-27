package main

import (
	"github.com/ryo-kagawa/go-utils/commandline"
)

type Command struct{}

var _ = (commandline.RootCommand)(Command{})

func (Command) Execute(arguments []string) (string, error) {
	cuePath := arguments[0]
	outputDirectory := arguments[1]
	cue, err := LoadCue(cuePath)
	if err != nil {
		return "", err
	}
	cue = cue.SplitTrack()
	err = cue.Output(outputDirectory)
	if err != nil {
		return "", err
	}

	return "", nil
}
