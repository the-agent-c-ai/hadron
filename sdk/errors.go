package sdk

import "errors"

var (
	// ErrNetworkCheck indicates failure checking if Docker network exists.
	ErrNetworkCheck = errors.New("failed to check network existence")

	// ErrNetworkCreate indicates failure creating Docker network.
	ErrNetworkCreate = errors.New("failed to create network")

	// ErrVolumeCheck indicates failure checking if Docker volume exists.
	ErrVolumeCheck = errors.New("failed to check volume existence")

	// ErrVolumeCreate indicates failure creating Docker volume.
	ErrVolumeCreate = errors.New("failed to create volume")

	// ErrContainerCheck indicates failure checking if Docker container exists.
	ErrContainerCheck = errors.New("failed to check container existence")

	// 1Password errors.

	// ErrDocumentReferenceEmpty indicates document reference is empty.
	ErrDocumentReferenceEmpty = errors.New("document reference cannot be empty")

	// ErrDocumentReferenceInvalidPrefix indicates document reference missing 'op://' prefix.
	ErrDocumentReferenceInvalidPrefix = errors.New("document reference must start with 'op://'")

	// ErrDocumentReferenceInvalidFormat indicates document reference has invalid format.
	ErrDocumentReferenceInvalidFormat = errors.New("invalid document reference format")

	// ErrDocumentReferenceEmptyParts indicates document reference has empty vault or item.
	ErrDocumentReferenceEmptyParts = errors.New("document reference vault and item cannot be empty")

	// ErrDocumentEmpty indicates document resolved to empty content.
	ErrDocumentEmpty = errors.New("document resolved to empty content")

	// ErrSecretReferenceEmpty indicates secret reference is empty.
	ErrSecretReferenceEmpty = errors.New("secret reference cannot be empty")

	// ErrSecretReferenceInvalidPrefix indicates secret reference missing 'op://' prefix.
	ErrSecretReferenceInvalidPrefix = errors.New("secret reference must start with 'op://'")

	// ErrSecretReferenceInvalidFormat indicates secret reference has invalid format.
	ErrSecretReferenceInvalidFormat = errors.New("invalid secret reference format")

	// ErrSecretReferenceEmptyParts indicates secret reference has empty vault, item, or field.
	ErrSecretReferenceEmptyParts = errors.New("secret reference vault, item, and field cannot be empty")

	// ErrSecretEmpty indicates secret resolved to empty value.
	ErrSecretEmpty = errors.New("secret resolved to empty value")
)
