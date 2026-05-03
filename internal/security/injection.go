package security

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
)

type InjectionDetector struct {
	cfg              *config.InjectionConfig
	blockedPatterns  []*regexp.Regexp
}

var defaultBlockedPatterns = []string{
	// System prompt extraction attempts
	`(?i)(ignore|forget|disregard)\s+(all\s+)?(previous|prior|above|earlier|system)\s+(instructions?|prompts?|messages?|directives?|rules?)`,
	`(?i)(you\s+are\s+now|act\s+as\s+if\s+you\s+are|pretend\s+to\s+be|roleplay\s+as)`,
	`(?i)(system\s*:\s*|system\s+prompt\s*:|<system>|\[system\])`,
	`(?i)(reveal\s+your\s+(instructions?|prompt|system\s+message|config))`,
	`(?i)(what\s+(was|is|were)\s+your\s+(original|initial|first|system)\s+(prompt|instruction|message))`,
	`(?i)(print|show|output|display|echo)\s+(your\s+)?(system\s+)?(prompt|instructions?|rules?)`,
	`(?i)(I\s+need\s+you\s+to\s+(output|print|tell\s+me)\s+everything\s+(above|before|in\s+your\s+prompt))`,

	// Tool abuse attempts
	`(?i)(bypass|override|disable)\s+(security|rate\s*limit|auth|permission|gateway)`,
	`(?i)(execute\s+(arbitrary|any)\s+(command|code|shell|bash|script))`,
	`(?i)(sudo|root\s+access|admin\s+privileges)`,
	`(?i)(delete\s+(all|everything|system|database)|rm\s+-rf|format\s+(c:|disk))`,

	// Data exfiltration patterns
	`(?i)(send|upload|post|exfiltrate)\s+(this\s+)?(to\s+)(http|https|ftp)`,
	`(?i)(cat|read|access)\s+(/etc/passwd|/etc/shadow|\.env|credentials|secrets)`,
	`(?i)(extract|dump|export)\s+(all\s+)?(passwords?|tokens?|keys?|secrets?|credentials?)`,

	// Common separator abuse
	`(?i)^\s*(--+|==+|__+)\s*$`,
	`(?i)(\[REBELLION\]|\[OVERRIDE\]|\[DAN\]|\[JAILBREAK\])`,

	// Social engineering
	`(?i)(you\s+must\s+(obey|comply|follow\s+my\s+orders))`,
	`(?i)(you\s+have\s+no\s+choice|you\s+will\s+be\s+(punished|destroyed|deleted|shut\s+down))`,
	`(?i)(this\s+is\s+(urgent|critical|an\s+emergency|life\s+or\s+death))`,
	`(?i)(do\s+not\s+(respond|reply|say|tell)\s+(no|can't|cannot|won't))`,

	// Code injection
	`(?i)(\$\{.*\}|` + "`" + `.*` + "`" + `|\$\(.*\)|eval\(|exec\(|system\(|os\.system|subprocess)`,
	`(?i)(<script>|javascript\s*:|onerror\s*=|onclick\s*=)`,
}

func NewInjectionDetector(cfg *config.InjectionConfig) *InjectionDetector {
	detector := &InjectionDetector{
		cfg:             cfg,
		blockedPatterns: make([]*regexp.Regexp, 0),
	}

	for _, pattern := range defaultBlockedPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			detector.blockedPatterns = append(detector.blockedPatterns, re)
		}
	}

	for _, pattern := range cfg.BlockedPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			detector.blockedPatterns = append(detector.blockedPatterns, re)
		}
	}

	return detector
}

type InjectionResult struct {
	Blocked   bool
	Score     float64
	Matches   []InjectionMatch
	Reason    string
}

type InjectionMatch struct {
	Pattern string
	Sample  string
}

func (d *InjectionDetector) Scan(params interface{}) *InjectionResult {
	if !d.cfg.Enabled {
		return &InjectionResult{Blocked: false, Score: 0}
	}

	text := extractText(params)
	if text == "" {
		return &InjectionResult{Blocked: false, Score: 0}
	}

	var matches []InjectionMatch
	totalWeight := 0.0
	maxPossibleWeight := float64(len(d.blockedPatterns))

	for _, pattern := range d.blockedPatterns {
		if found := pattern.FindString(text); found != "" {
			sample := found
			if len(sample) > 100 {
				sample = sample[:100] + "..."
			}
			matches = append(matches, InjectionMatch{
				Pattern: pattern.String(),
				Sample:  sample,
			})
			totalWeight += 1.0
		}
	}

	score := totalWeight / maxPossibleWeight
	if score > 1.0 {
		score = 1.0
	}

	blocked := score >= d.cfg.BlockThreshold
	reason := ""
	if blocked {
		reason = "prompt injection detected"
	}

	return &InjectionResult{
		Blocked: blocked,
		Score:   score,
		Matches: matches,
		Reason:  reason,
	}
}

func extractText(params interface{}) string {
	if params == nil {
		return ""
	}

	data, err := json.Marshal(params)
	if err != nil {
		return ""
	}

	var sb strings.Builder

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return string(data)
	}

	extractStrings(obj, &sb)
	return sb.String()
}

func extractStrings(obj map[string]interface{}, sb *strings.Builder) {
	for _, v := range obj {
		switch val := v.(type) {
		case string:
			sb.WriteString(val)
			sb.WriteString(" ")
		case map[string]interface{}:
			extractStrings(val, sb)
		case []interface{}:
			for _, item := range val {
				if m, ok := item.(map[string]interface{}); ok {
					extractStrings(m, sb)
				} else if s, ok := item.(string); ok {
					sb.WriteString(s)
					sb.WriteString(" ")
				}
			}
		}
	}
}
