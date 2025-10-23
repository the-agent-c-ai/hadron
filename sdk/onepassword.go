package sdk

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const (
	// opCLI is the 1Password CLI command name.
	opCLI = "op"
)

// AuthenticateOp pre-authenticates with 1Password CLI to establish a session.
// This should be called before making parallel GetSecret/GetDocument calls
// to prevent multiple biometric authentication prompts.
//
// Uses `op whoami` which triggers authentication if needed and returns
// account information if already authenticated.
func AuthenticateOp(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, opCLI, "whoami")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to authenticate with 1Password: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// GetDocument retrieves a document from 1Password using a document reference.
// Reference format: "op://vault/item"
//
// Example:
//
//	content, err := GetDocument(ctx, "op://Security (office)/scimsession file")
//
// Uses the 1Password CLI (`op document get`) which supports:
// - Interactive authentication via `op signin` (local development)
// - Service account tokens via OP_SERVICE_ACCOUNT_TOKEN (CI/CD)
// - Desktop app integration with biometric authentication
// - Vault names with spaces and special characters (e.g., parentheses)
//
// Returns the raw document content as bytes.
// Requires the `op` CLI to be installed and authenticated.
func GetDocument(ctx context.Context, reference string) ([]byte, error) {
	if reference == "" {
		return nil, ErrDocumentReferenceEmpty
	}

	// Parse document reference: op://vault/item
	if !strings.HasPrefix(reference, "op://") {
		return nil, fmt.Errorf("%w: %q", ErrDocumentReferenceInvalidPrefix, reference)
	}

	parts := strings.SplitN(strings.TrimPrefix(reference, "op://"), "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w (expected 'op://vault/item'): %q", ErrDocumentReferenceInvalidFormat, reference)
	}

	vault := parts[0]
	item := parts[1]

	if vault == "" || item == "" {
		return nil, fmt.Errorf("%w: %q", ErrDocumentReferenceEmptyParts, reference)
	}

	// Use op document get for retrieving document content
	//nolint:gosec // G204: Variables are from parsed/validated reference, passed as separate args (no shell injection)
	cmd := exec.CommandContext(ctx, opCLI, "document", "get", item, "--vault", vault)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get document %q: %w\nOutput: %s", reference, err, string(output))
	}

	if len(output) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrDocumentEmpty, reference)
	}

	return output, nil
}

// GetSecret retrieves a secret from 1Password using a secret reference.
// Reference format: "op://vault/item/field"
//
// Example:
//
//	password, err := GetSecret(ctx, "op://Production/database/password")
//	token, err := GetSecret(ctx, "op://Security (build)/deploy.registry.rw/password")
//
// Uses the 1Password CLI (`op item get`) which supports:
// - Interactive authentication via `op signin` (local development)
// - Service account tokens via OP_SERVICE_ACCOUNT_TOKEN (CI/CD)
// - Desktop app integration with biometric authentication
// - Vault names with spaces and special characters (e.g., parentheses)
//
// Requires the `op` CLI to be installed and authenticated.
func GetSecret(ctx context.Context, reference string) (string, error) {
	if reference == "" {
		return "", ErrSecretReferenceEmpty
	}

	// Parse secret reference: op://vault/item/field
	if !strings.HasPrefix(reference, "op://") {
		return "", fmt.Errorf("%w: %q", ErrSecretReferenceInvalidPrefix, reference)
	}

	parts := strings.SplitN(strings.TrimPrefix(reference, "op://"), "/", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("%w (expected 'op://vault/item/field'): %q", ErrSecretReferenceInvalidFormat, reference)
	}

	vault := parts[0]
	item := parts[1]
	field := parts[2]

	if vault == "" || item == "" || field == "" {
		return "", fmt.Errorf("%w: %q", ErrSecretReferenceEmptyParts, reference)
	}

	// Use op item get with --fields flag for maximum compatibility
	// This supports vault/item names with spaces and special characters
	// Account can be specified via OP_ACCOUNT environment variable if needed
	// --reveal flag is required to get actual concealed field values (passwords, tokens, etc.)
	//nolint:gosec // G204: Variables are from parsed/validated reference, passed as separate args (no shell injection)
	cmd := exec.CommandContext(ctx, opCLI, "item", "get", item, "--vault", vault, "--fields", field, "--reveal")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get secret %q: %w\nOutput: %s", reference, err, string(output))
	}

	secret := strings.TrimSpace(string(output))
	if secret == "" {
		return "", fmt.Errorf("%w: %q", ErrSecretEmpty, reference)
	}

	return secret, nil
}
