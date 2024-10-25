package labels

import "fmt"

const (
	In           Operator = "In"
	NotIn        Operator = "NotIn"
	Exists       Operator = "Exists"
	DoesNotExist Operator = "DoesNotExist"
)

const DefaultLabelPrefix = "blipblop.io"

var DefaultContainerLabels = []string{LabelPrefix("uuid").String()}

type LabelPrefix string

func (d LabelPrefix) String() string {
	return fmt.Sprintf("%s/%s", DefaultLabelPrefix, string(d))
}

// Label is a map representing metadata of each resource.
type Label map[string]string

func (l *Label) Get(key string) string {
	if val, ok := (*l)[key]; ok {
		return val
	}
	return ""
}

func (l *Label) Set(key string, value string) {
	(*l)[key] = value
}

func (l *Label) Delete(key string) {
	delete(*l, key)
}

func (l *Label) AppendMap(m map[string]string) {
	for k, v := range m {
		l.Set(k, v)
	}
}

type Operator string

// Selector represents a filter based on key-value conditions.
type Selector struct {
	MatchLabels Label
	Expressions []LabelExpression
}

type LabelExpression struct {
	Key      string
	Operator Operator
	Values   []string
}

type LabelSelector interface {
	Matches(labels Label) bool
	AddMatchLabel(key, value string)
	AddExpression(expr LabelExpression)
}

func (s *Selector) Matches(labels Label) bool {
	// Match by key-value pairs in MatchLabels
	for key, value := range s.MatchLabels {
		if labels[key] != value {
			return false
		}
	}

	// Evaluate label expressions
	for _, expr := range s.Expressions {
		if !evaluateExpression(expr, labels) {
			return false
		}
	}

	return true
}

func evaluateExpression(expr LabelExpression, labels Label) bool {
	switch expr.Operator {
	case In:
		for _, v := range expr.Values {
			if labels[expr.Key] == v {
				return true
			}
		}
		return false
	case NotIn:
		for _, v := range expr.Values {
			if labels[expr.Key] == v {
				return false
			}
		}
		return true
	case Exists:
		_, exists := labels[expr.Key]
		return exists
	case DoesNotExist:
		_, exists := labels[expr.Key]
		return !exists
	default:
		return false
	}
}

func New() Label {
	return Label{}
}
