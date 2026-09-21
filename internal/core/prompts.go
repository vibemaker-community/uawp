package core

import (
	"fmt"

	promptassets "github.com/uawp/uawp/prompts"
)

type PromptName string

const (
	ResumeWork       PromptName = "RESUME_WORK"
	ContextSync      PromptName = "CONTEXT_SYNC"
	CreateCheckpoint PromptName = "CREATE_CHECKPOINT"
	PauseAndHandoff  PromptName = "PAUSE_AND_HANDOFF"
)

var promptNames = []PromptName{ResumeWork, ContextSync, CreateCheckpoint, PauseAndHandoff}

func PromptNames() []PromptName { return append([]PromptName(nil), promptNames...) }
func Prompt(name PromptName) ([]byte, error) {
	valid := false
	for _, candidate := range promptNames {
		if name == candidate {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("unknown prompt %q", name)
	}
	content, err := promptassets.FS.ReadFile(string(name) + ".md")
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), content...), nil
}
