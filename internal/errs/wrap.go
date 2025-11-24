package errs

import "fmt"

func Wrap(base, ext error) error {
	if ext == nil {
		return base
	}

	return fmt.Errorf("%w: %w", base, ext)
}

func Wrapf(base error, str string) error {
	return fmt.Errorf("%w: %s", base, str)
}
