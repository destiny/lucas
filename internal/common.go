package internal

type FnModeOptions struct {
	Debug bool
	Test  bool
}

type FnModeOption func(*FnModeOptions)

func WithDebug(debug bool) FnModeOption {
	return func(opts *FnModeOptions) {
		opts.Debug = debug
	}
}

func WithTest(test bool) FnModeOption {
	return func(opts *FnModeOptions) {
		opts.Test = test
	}
}

func NewModeOptions(options ...FnModeOption) *FnModeOptions {
	opts := &FnModeOptions{
		Debug: false,
		Test:  false,
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}
