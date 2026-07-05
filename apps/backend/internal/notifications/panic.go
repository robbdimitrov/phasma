package notifications

import "fmt"

func errPanic(value any) error {
	return fmt.Errorf("panic: %v", value)
}
