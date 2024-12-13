package air

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrNotAIR = errors.New("air: the value is not an AIR")

	regex = regexp.MustCompile(`^urn:air:(?P<Ecosystem>[^:]+):(?P<Type>[^:]+):(?P<Source>[^:]+):(?P<ID>[^@]+)@?(?P<Version>[^:]*):?(?P<Layer>[^.]*).?(?P<Format>[^?]*)$`)
)

type ID struct {
	Ecosystem    string
	Type         string
	Source       string
	ModelID      string
	ModelVersion string
	Layer        string
	Format       string
}

func (id ID) String() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "urn:air:%s:%s:%s:%s@%s", id.Ecosystem, id.Type, id.Source, id.ModelID, id.ModelVersion)

	return sb.String()
}

func Parse(inp string) (*ID, error) {
	if !strings.HasPrefix(inp, "urn:air:") {
		return nil, ErrNotAIR
	}

	var result ID

	match := regex.FindStringSubmatch(inp)

	if match != nil {
		result = ID{
			Ecosystem:    match[1],
			Type:         match[2],
			Source:       match[3],
			ModelID:      match[4],
			ModelVersion: match[5],
			Layer:        match[6],
			Format:       match[7],
		}
	} else {
		return nil, ErrNotAIR
	}

	return &result, nil
}
