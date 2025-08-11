// Copyright 2025 Arion Yau
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
