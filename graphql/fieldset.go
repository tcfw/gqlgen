package graphql

import (
	"io"
	"sync"
)

type FieldSet struct {
	fields  []CollectedField
	Values  []Marshaler
	delayed []delayedResult
	defered map[int]CollectedField
}

type delayedResult struct {
	i int
	f func() Marshaler
}

func NewFieldSet(fields []CollectedField) *FieldSet {
	return &FieldSet{
		fields:  fields,
		Values:  make([]Marshaler, len(fields)),
		defered: make(map[int]CollectedField),
	}
}

func (m *FieldSet) Concurrently(i int, f func() Marshaler) {
	m.delayed = append(m.delayed, delayedResult{i: i, f: f})
}

func (m *FieldSet) Defered(i int, f CollectedField) {
	m.defered[i] = f
}

func (m *FieldSet) Dispatch() {
	if len(m.delayed) == 1 {
		// only one concurrent task, no need to spawn a goroutine or deal create waitgroups
		d := m.delayed[0]
		m.Values[d.i] = d.f()
	} else if len(m.delayed) > 1 {
		// more than one concurrent task, use the main goroutine to do one, only spawn goroutines for the others

		var wg sync.WaitGroup
		for _, d := range m.delayed[1:] {
			wg.Add(1)
			go func(d delayedResult) {
				m.Values[d.i] = d.f()
				wg.Done()
			}(d)
		}

		m.Values[m.delayed[0].i] = m.delayed[0].f()
		wg.Wait()
	}
}

func (m *FieldSet) MarshalGQL(writer io.Writer) {
	writer.Write(openBrace)
	var prevDefered bool
	for i, field := range m.fields {
		if _, ok := m.defered[i]; ok {
			prevDefered = true
			continue
		}

		if i != 0 && !prevDefered {
			writer.Write(comma)
		}
		prevDefered = false

		writeQuotedString(writer, field.Alias)
		writer.Write(colon)
		m.Values[i].MarshalGQL(writer)
	}
	writer.Write(closeBrace)
}
