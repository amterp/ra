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
