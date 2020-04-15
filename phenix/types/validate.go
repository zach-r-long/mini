package types

import (
	"fmt"

	"phenix/types/version"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
)

func ValidateConfigSpec(c Config) error {
	if g := c.APIGroup(); g != API_GROUP {
		return fmt.Errorf("invalid API group %s: expected %s", g, API_GROUP)
	}

	v, err := version.GetVersionedValidatorForKind(c.Kind, c.APIVersion())
	if err != nil {
		return fmt.Errorf("getting validator for config: %w", err)
	}

	if err := validate.AgainstSchema(v, c.Spec, strfmt.Default); err != nil {
		return fmt.Errorf("validating config spec against schema: %w", err)
	}

	return nil
}