package ra

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func getBaseFlag(flag any) *BaseFlag {
	switch f := flag.(type) {
	case *BoolFlag:
		return &f.BaseFlag
	case *StringFlag:
		return &f.BaseFlag
	case *IntFlag:
		return &f.BaseFlag
	case *Int64Flag:
		return &f.BaseFlag
	case *Float64Flag:
		return &f.BaseFlag
	case *StringSliceFlag:
		return &f.BaseFlag
	case *IntSliceFlag:
		return &f.BaseFlag
	case *Int64SliceFlag:
		return &f.BaseFlag
	case *Float64SliceFlag:
		return &f.BaseFlag
	case *BoolSliceFlag:
		return &f.BaseFlag
	}
	return nil
}

func isSliceFlag(flag any) bool {
	switch flag.(type) {
	case *StringSliceFlag, *IntSliceFlag, *Int64SliceFlag, *Float64SliceFlag, *BoolSliceFlag:
		return true
	}
	return false
}

// isVariadicFlag reports flag-side greed: a variadic flag collects a run of
// values after its own flag token ("--options a b c"), where a plain list takes
// one value per occurrence ("--options a --options b").
func isVariadicFlag(flag any) bool {
	switch f := flag.(type) {
	case *StringSliceFlag:
		return f.Variadic
	case *IntSliceFlag:
		return f.Variadic
	case *Int64SliceFlag:
		return f.Variadic
	case *Float64SliceFlag:
		return f.Variadic
	case *BoolSliceFlag:
		return f.Variadic
	}
	return false
}

// absorbsPositionals reports positional-side greed: once a slice flag starts
// collecting bare positional values it takes every remaining one, so nothing
// declared after it can be filled positionally. True for any positional-capable
// slice, variadic or not - unlike isVariadicFlag, which is about the flag form.
func absorbsPositionals(flag any) bool {
	base := getBaseFlag(flag)
	return isSliceFlag(flag) && base != nil && !base.FlagOnly
}

func deepCopyFlag(flag any) any {
	switch f := flag.(type) {
	case *BoolFlag:
		copy := *f
		return &copy
	case *StringFlag:
		copy := *f
		return &copy
	case *IntFlag:
		copy := *f
		return &copy
	case *Int64Flag:
		copy := *f
		return &copy
	case *Float64Flag:
		copy := *f
		return &copy
	case *StringSliceFlag:
		copy := *f
		return &copy
	case *IntSliceFlag:
		copy := *f
		return &copy
	case *Int64SliceFlag:
		copy := *f
		return &copy
	case *Float64SliceFlag:
		copy := *f
		return &copy
	case *BoolSliceFlag:
		copy := *f
		return &copy
	}
	return nil
}
