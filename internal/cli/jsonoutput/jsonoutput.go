package jsonoutput

import (
	"encoding/json"
	"fmt"
	"io"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

const ContractVersion = "v1"
const ContractName = "bbsc.machine"

type Envelope struct {
	Version string       `json:"version"`
	Data    any          `json:"data"`
	Meta    EnvelopeMeta `json:"meta"`
}

type EnvelopeMeta struct {
	Contract string `json:"contract"`
}

func Write(writer io.Writer, payload any) error {
	envelope := Envelope{
		Version: ContractVersion,
		Data:    payload,
		Meta: EnvelopeMeta{
			Contract: ContractName,
		},
	}

	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to encode JSON output", err)
	}

	if _, err := fmt.Fprintln(writer, string(encoded)); err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to write JSON output", err)
	}

	return nil
}
