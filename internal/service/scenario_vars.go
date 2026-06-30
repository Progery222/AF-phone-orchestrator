package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type scenarioVariables struct {
	BrowserResearch map[string]any `yaml:"browser_research"`
	WarmupProfiles  map[string]any `yaml:"warmup_profiles"`
	WarmupFeed      map[string]any `yaml:"warmup_feed"`
	Publish         map[string]any `yaml:"publish"`
}

func parseScenarioVariables(raw string) (scenarioVariables, error) {
	var v scenarioVariables
	if strings.TrimSpace(raw) == "" {
		return v, nil
	}
	if err := yaml.Unmarshal([]byte(raw), &v); err != nil {
		return scenarioVariables{}, fmt.Errorf("variables.yaml: %w", err)
	}
	return v, nil
}

func pickRange(rng *rand.Rand, v any, fallback int) int {
	switch t := v.(type) {
	case []any:
		if len(t) >= 2 {
			return rng.Intn(intVal(t[1])-intVal(t[0])+1) + intVal(t[0])
		}
	case []int:
		if len(t) >= 2 {
			return rng.Intn(t[1]-t[0]+1) + t[0]
		}
	case int:
		return t
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return fallback
}

func pickFloatRange(rng *rand.Rand, v any, fallback float64) float64 {
	switch t := v.(type) {
	case []any:
		if len(t) >= 2 {
			lo, hi := floatVal(t[0]), floatVal(t[1])
			return lo + rng.Float64()*(hi-lo)
		}
	case []float64:
		if len(t) >= 2 {
			return t[0] + rng.Float64()*(t[1]-t[0])
		}
	case float64:
		return t
	case int:
		return float64(t)
	}
	return fallback
}

func intVal(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func floatVal(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return n
	default:
		return 0
	}
}

func asStringAnyMap(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		return t, true
	case map[interface{}]interface{}:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func nestedMap(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		raw, ok := cur[k]
		if !ok {
			return nil
		}
		next, ok := asStringAnyMap(raw)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

func mergeWarmupFeedVars(vars scenarioVariables, profile, phase string) map[string]any {
	feedVars := map[string]any{}
	for k, v := range vars.WarmupFeed {
		feedVars[k] = v
	}
	if profile == "" || phase == "" || vars.WarmupProfiles == nil {
		return feedVars
	}
	prof, ok := vars.WarmupProfiles[profile]
	if !ok {
		return feedVars
	}
	profMap, ok := asStringAnyMap(prof)
	if !ok {
		return feedVars
	}
	phaseRaw, ok := profMap[phase]
	if !ok {
		return feedVars
	}
	phaseCfg, ok := asStringAnyMap(phaseRaw)
	if !ok {
		return feedVars
	}
	for k, v := range phaseCfg {
		feedVars[k] = v
	}
	return feedVars
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func nestedSlice(m map[string]any, keys ...string) []string {
	cur := m
	for i, k := range keys {
		if cur == nil {
			return nil
		}
		raw, ok := cur[k]
		if !ok {
			return nil
		}
		if i == len(keys)-1 {
			switch t := raw.(type) {
			case []any:
				out := make([]string, 0, len(t))
				for _, item := range t {
					if s, ok := item.(string); ok && s != "" {
						out = append(out, s)
					}
				}
				return out
			case []string:
				return t
			}
			return nil
		}
		next, ok := asStringAnyMap(raw)
		if !ok {
			return nil
		}
		cur = next
	}
	return nil
}
