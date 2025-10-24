package mock

import (
	"encoding/pem"
	"io"

	"github.com/openkcm/cmk-core/internal/errs"
)

// PEMEncoder is a mock implementation of the PEMEncoder interface.
type PEMEncoder struct {
	counter           int
	ShouldReturnError int
}

// Encode writes the PEM encoding of block to out.
func (e *PEMEncoder) Encode(out io.Writer, block *pem.Block) error {
	if e.counter == e.ShouldReturnError {
		e.counter++
		return ErrForcedError
	}

	e.counter++

	err := pem.Encode(out, block)
	if err != nil {
		return errs.Wrap(ErrForcedError, err)
	}

	return nil
}
