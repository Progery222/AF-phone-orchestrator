package service

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type scenarioStepRef struct {
	ID     string            `yaml:"id"`
	Action string            `yaml:"action"`
	Uses   string            `yaml:"uses"`
	Params map[string]string `yaml:"params"`
}

type scenarioStepsDoc struct {
	Steps []scenarioStepRef `yaml:"steps"`
}

// LookupStepFromYAML находит шаг по id и возвращает action, uses, params.
func LookupStepFromYAML(scenarioYAML, stepID string) (action, uses string, params map[string]string, err error) {
	var doc scenarioStepsDoc
	if err := yaml.Unmarshal([]byte(scenarioYAML), &doc); err != nil {
		return "", "", nil, fmt.Errorf("scenario.yaml: %w", err)
	}
	for _, st := range doc.Steps {
		if st.ID == stepID {
			p := st.Params
			if p == nil {
				p = map[string]string{}
			}
			return st.Action, st.Uses, p, nil
		}
	}
	return "", "", nil, fmt.Errorf("шаг %q не найден в scenario.yaml", stepID)
}

// MergeStepRequest заполняет action/uses/params из YAML, если не заданы в запросе.
func MergeStepRequest(scenarioYAML, stepID, action, uses string, params map[string]string) (string, string, map[string]string, error) {
	if strings.TrimSpace(scenarioYAML) == "" || stepID == "" {
		return action, uses, params, nil
	}
	yAction, yUses, yParams, err := LookupStepFromYAML(scenarioYAML, stepID)
	if err != nil {
		return action, uses, params, err
	}
	if action == "" {
		action = yAction
	}
	if uses == "" {
		uses = yUses
	}
	if len(params) == 0 {
		params = yParams
	}
	return action, uses, params, nil
}
