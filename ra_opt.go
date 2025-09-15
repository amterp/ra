package ra

type RegisterOption func(*registerConfig)

type registerConfig struct {
	global           bool
	bypassValidation bool
}

func WithGlobal(g bool) RegisterOption {
	return func(c *registerConfig) {
		c.global = g
	}
}

func WithBypassValidation(b bool) RegisterOption {
	return func(c *registerConfig) {
		c.bypassValidation = b
	}
}

type parseCfg struct {
	ignoreUnknown        bool
	variadicUnknownFlags bool
	dump                 bool
}

type ParseOpt func(*parseCfg)

func WithIgnoreUnknown(ignore bool) ParseOpt {
	return func(c *parseCfg) {
		c.ignoreUnknown = ignore
	}
}

func WithVariadicUnknownFlags(enable bool) ParseOpt {
	return func(c *parseCfg) {
		c.variadicUnknownFlags = enable
	}
}

func WithDump(dump bool) ParseOpt {
	return func(c *parseCfg) {
		c.dump = dump
	}
}
