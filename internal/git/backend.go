package git

import "context"

type CloneOptions struct {
	Directory string
	Branch    string
	Depth     int
}

type FetchOptions struct {
	Remote string
}

type CheckoutOptions struct {
	Ref string
}

type Backend interface {
	Version(ctx context.Context) (string, error)
	Clone(ctx context.Context, repositoryURL string, options CloneOptions) error
	Fetch(ctx context.Context, repositoryDirectory string, options FetchOptions) error
	Checkout(ctx context.Context, repositoryDirectory string, options CheckoutOptions) error
}
