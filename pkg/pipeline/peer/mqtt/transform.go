package mqtt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type TopicToField struct {
	Field   string `json:"field" yaml:"field"`
	Index   string `json:"index,omitempty" yaml:"index,omitempty"` // "0", "1", "-1", etc.
	Pattern string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Static  string `json:"static,omitempty" yaml:"static,omitempty"`
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
}

func extractFieldsFromTopic(topic string, fields []TopicToField) map[string]any {
	segments := strings.Split(strings.Trim(topic, "/"), "/")
	result := make(map[string]any)

	for _, field := range fields {
		var rawValue string

		// Static value
		if field.Static != "" {
			rawValue = field.Static
		} else if field.Index != "" {
			// Index-based extraction - parse string to int
			if idx, err := strconv.Atoi(field.Index); err == nil {
				if idx < 0 {
					idx = len(segments) + idx // Support negative indices
				}
				if idx >= 0 && idx < len(segments) {
					rawValue = segments[idx]
				}
			}
		} else if field.Pattern != "" {
			// Regex extraction
			if re, err := regexp.Compile(field.Pattern); err == nil {
				if matches := re.FindStringSubmatch(topic); len(matches) > 1 {
					rawValue = matches[1]
				}
			}
		}

		// Skip empty values
		if rawValue == "" {
			continue
		}

		// Type conversion
		result[field.Field] = parseScalarValue(rawValue, field.Type)
	}

	return result
}

func parseScalarValue(value string, fieldType ...string) any {
	if len(fieldType) > 0 {
		switch fieldType[0] {
		case "bool":
			if v, err := strconv.ParseBool(value); err == nil {
				return v
			}
		case "int":
			if v, err := strconv.Atoi(value); err == nil {
				return v
			}
		case "float":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				return v
			}
		}
		return value
	}

	// Auto-detect: try bool, int, float, then string
	if v, err := strconv.ParseBool(value); err == nil {
		return v
	}
	if v, err := strconv.ParseInt(value, 10, 64); err == nil {
		return v
	}
	if v, err := strconv.ParseFloat(value, 64); err == nil {
		return v
	}
	return value
}

type RewriteAction string

const (
	ActionPubSub    RewriteAction = "pubsub"
	ActionPublish   RewriteAction = "pub"
	ActionSubscribe RewriteAction = "sub"
)

type TopicRewriteRule struct {
	Action     RewriteAction  `json:"action" yaml:"action"`
	From       string         `json:"from" yaml:"from"`
	To         string         `json:"to" yaml:"to"`
	Regex      string         `json:"regex" yaml:"regex"`
	compiledRe *regexp.Regexp `json:"-" yaml:"-"`
}

type TopicRewriter struct {
	rules []TopicRewriteRule
}

func NewTopicRewriter(rules []TopicRewriteRule) (*TopicRewriter, error) {
	tr := &TopicRewriter{rules: make([]TopicRewriteRule, len(rules))}
	for i, rule := range rules {
		var err error
		if rule.compiledRe, err = regexp.Compile(rule.Regex); err != nil {
			return nil, fmt.Errorf("invalid regex in rule %d: %v", i, err)
		}
		tr.rules[i] = rule
	}
	return tr, nil
}

func topicMatch(pattern, topic string) bool {
	return matchParts(strings.Split(pattern, "/"), strings.Split(topic, "/"), 0, 0)
}

func matchParts(pattern, topic []string, pIdx, tIdx int) bool {
	if pIdx >= len(pattern) {
		return tIdx >= len(topic)
	}
	if tIdx >= len(topic) {
		return pIdx == len(pattern)-1 && pattern[pIdx] == "#"
	}
	switch pattern[pIdx] {
	case "#":
		return true
	case "+":
		return matchParts(pattern, topic, pIdx+1, tIdx+1)
	default:
		return pattern[pIdx] == topic[tIdx] && matchParts(pattern, topic, pIdx+1, tIdx+1)
	}
}

func (tr *TopicRewriter) RewriteTopic(topic string, action RewriteAction, clientID, username string) string {
	if tr == nil || len(tr.rules) == 0 {
		return topic
	}

	for _, rule := range tr.rules {
		if rule.Action != ActionPubSub && rule.Action != action {
			continue
		}
		if !topicMatch(rule.From, topic) {
			continue
		}
		matches := rule.compiledRe.FindStringSubmatch(topic)
		if matches == nil {
			break
		}

		dest := rule.To
		for i := 1; i < len(matches); i++ {
			dest = strings.ReplaceAll(dest, fmt.Sprintf("$%d", i), matches[i])
		}
		dest = strings.ReplaceAll(dest, "${clientid}", clientID)
		dest = strings.ReplaceAll(dest, "${username}", username)
		return dest
	}
	return topic
}
