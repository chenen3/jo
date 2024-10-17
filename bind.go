package main

type bindStr struct {
	value    string
	onChange func()
}

func BindStr(s string, onChange func()) *bindStr {
	return &bindStr{value: s, onChange: onChange}
}

func (b *bindStr) Get() string { return b.value }

func (b *bindStr) Set(v string) {
	b.value = v
	if b.onChange != nil {
		b.onChange()
	}
}
