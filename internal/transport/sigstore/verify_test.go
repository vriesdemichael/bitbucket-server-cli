package sigstore

import (
	"bytes"
	"context"
	"testing"

	sigroot "github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	sigverify "github.com/sigstore/sigstore-go/pkg/verify"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

func TestVerifierVerifyEntity(t *testing.T) {
	virtualSigstore, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatalf("NewVirtualSigstore returned error: %v", err)
	}
	artifact := []byte("signed checksum manifest")
	entity, err := virtualSigstore.Sign("foo!oidc.local", "http://oidc.local:8080", artifact)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	verifier := NewVerifier(Options{
		ExpectedIssuer: "http://oidc.local:8080",
		ExpectedSAN:    "foo!oidc.local",
		TrustedMaterialProvider: func(context.Context) (sigroot.TrustedMaterial, error) {
			return virtualSigstore, nil
		},
		VerifierOptions: []sigverify.VerifierOption{
			sigverify.WithTransparencyLog(1),
			sigverify.WithObserverTimestamps(1),
		},
	})

	verification, err := verifier.verifyEntity(context.Background(), entity, sigverify.WithArtifact(bytes.NewReader(artifact)))
	if err != nil {
		t.Fatalf("verifyEntity returned error: %v", err)
	}
	if verification.CertificateIdentity != "foo!oidc.local" {
		t.Fatalf("unexpected certificate identity: %+v", verification)
	}
	if verification.CertificateOIDCIssuer != "http://oidc.local:8080" {
		t.Fatalf("unexpected certificate issuer: %+v", verification)
	}
	if verification.TransparencyLogEntriesVerified == 0 {
		t.Fatalf("expected Rekor verification evidence, got %+v", verification)
	}
	if verification.VerifiedTimestampCount == 0 {
		t.Fatalf("expected verified timestamps, got %+v", verification)
	}
}

func TestNewVerifierDefaults(t *testing.T) {
	verifier := NewVerifier(Options{ExpectedIssuer: " https://issuer.example ", ExpectedSAN: " https://san.example "})
	if verifier == nil {
		t.Fatal("expected verifier")
	}
	if verifier.expectedIssuer != "https://issuer.example" {
		t.Fatalf("expected trimmed issuer, got %q", verifier.expectedIssuer)
	}
	if verifier.expectedSAN != "https://san.example" {
		t.Fatalf("expected trimmed san, got %q", verifier.expectedSAN)
	}
	if verifier.trustedMaterialProvider == nil {
		t.Fatal("expected trusted material provider")
	}
	if len(verifier.verifierOptions) != 3 {
		t.Fatalf("expected default verifier options, got %d", len(verifier.verifierOptions))
	}
}

func TestNewVerifierUsesExplicitProviderAndOptions(t *testing.T) {
	providerCalls := 0
	provider := func(context.Context) (sigroot.TrustedMaterial, error) {
		providerCalls++
		return nil, nil
	}
	options := []sigverify.VerifierOption{sigverify.WithTransparencyLog(1)}

	verifier := NewVerifier(Options{
		ExpectedIssuer:          GitHubActionsIssuer,
		ExpectedSAN:             "https://github.com/example/repo/.github/workflows/release.yml@refs/heads/main",
		TrustedMaterialProvider: provider,
		VerifierOptions:         options,
	})

	if verifier.trustedMaterialProvider == nil {
		t.Fatal("expected trusted material provider")
	}
	if got := len(verifier.verifierOptions); got != len(options) {
		t.Fatalf("expected %d verifier options, got %d", len(options), got)
	}
	if verifier.expectedIssuer != GitHubActionsIssuer {
		t.Fatalf("unexpected issuer: %q", verifier.expectedIssuer)
	}
	if _, err := verifier.trustedMaterialProvider(context.Background()); err != nil {
		t.Fatalf("trustedMaterialProvider returned error: %v", err)
	}
	if providerCalls != 1 {
		t.Fatalf("expected provider to be invoked once, got %d", providerCalls)
	}
}

func TestNewGitHubReleaseVerifier(t *testing.T) {
	verifier := NewGitHubReleaseVerifier(" vriesdemichael ", " bitbucket-server-cli ")
	if verifier == nil {
		t.Fatal("expected verifier")
	}
	if verifier.expectedIssuer != GitHubActionsIssuer {
		t.Fatalf("expected GitHub issuer, got %q", verifier.expectedIssuer)
	}
	expectedSAN := "https://github.com/vriesdemichael/bitbucket-server-cli/.github/workflows/release.yml@refs/heads/main"
	if verifier.expectedSAN != expectedSAN {
		t.Fatalf("expected SAN %q, got %q", expectedSAN, verifier.expectedSAN)
	}
}

func TestVerifierVerifyBlobErrors(t *testing.T) {
	t.Run("nil verifier", func(t *testing.T) {
		var verifier *Verifier
		_, err := verifier.VerifyBlob(context.Background(), []byte("artifact"), []byte("bundle"))
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})

	t.Run("invalid bundle", func(t *testing.T) {
		verifier := NewVerifier(Options{ExpectedIssuer: GitHubActionsIssuer, ExpectedSAN: "san"})
		_, err := verifier.VerifyBlob(context.Background(), []byte("artifact"), []byte("not-json"))
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("missing artifact content", func(t *testing.T) {
		verifier := NewVerifier(Options{ExpectedIssuer: GitHubActionsIssuer, ExpectedSAN: "san"})
		_, err := verifier.VerifyBlob(context.Background(), nil, []byte("bundle"))
		if !apperrors.IsKind(err, apperrors.KindValidation) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("missing bundle content", func(t *testing.T) {
		verifier := NewVerifier(Options{ExpectedIssuer: GitHubActionsIssuer, ExpectedSAN: "san"})
		_, err := verifier.VerifyBlob(context.Background(), []byte("artifact"), nil)
		if !apperrors.IsKind(err, apperrors.KindValidation) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("missing expected identity configuration", func(t *testing.T) {
		verifier := NewVerifier(Options{})
		_, err := verifier.VerifyBlob(context.Background(), []byte("artifact"), []byte("bundle"))
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})

	t.Run("trusted root failure bubbles up", func(t *testing.T) {
		virtualSigstore, err := ca.NewVirtualSigstore()
		if err != nil {
			t.Fatalf("NewVirtualSigstore returned error: %v", err)
		}
		artifact := []byte("artifact")
		entity, err := virtualSigstore.Sign("foo!oidc.local", GitHubActionsIssuer, artifact)
		if err != nil {
			t.Fatalf("Sign returned error: %v", err)
		}
		verifier := NewVerifier(Options{
			ExpectedIssuer: GitHubActionsIssuer,
			ExpectedSAN:    "foo!oidc.local",
			TrustedMaterialProvider: func(context.Context) (sigroot.TrustedMaterial, error) {
				return nil, apperrors.New(apperrors.KindTransient, "boom", nil)
			},
		})
		_, err = verifier.verifyEntity(context.Background(), entity, sigverify.WithArtifact(bytes.NewReader(artifact)))
		if !apperrors.IsKind(err, apperrors.KindTransient) {
			t.Fatalf("expected transient error, got %v", err)
		}
	})

	t.Run("identity mismatch", func(t *testing.T) {
		virtualSigstore, err := ca.NewVirtualSigstore()
		if err != nil {
			t.Fatalf("NewVirtualSigstore returned error: %v", err)
		}
		artifact := []byte("artifact")
		entity, err := virtualSigstore.Sign("foo!oidc.local", "http://oidc.local:8080", artifact)
		if err != nil {
			t.Fatalf("Sign returned error: %v", err)
		}

		verifier := NewVerifier(Options{
			ExpectedIssuer: "http://oidc.local:8080",
			ExpectedSAN:    "other@example.com",
			TrustedMaterialProvider: func(context.Context) (sigroot.TrustedMaterial, error) {
				return virtualSigstore, nil
			},
			VerifierOptions: []sigverify.VerifierOption{
				sigverify.WithTransparencyLog(1),
				sigverify.WithObserverTimestamps(1),
			},
		})

		_, err = verifier.verifyEntity(context.Background(), entity, sigverify.WithArtifact(bytes.NewReader(artifact)))
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("nil signed entity", func(t *testing.T) {
		verifier := NewVerifier(Options{ExpectedIssuer: GitHubActionsIssuer, ExpectedSAN: "san", TrustedMaterialProvider: func(context.Context) (sigroot.TrustedMaterial, error) {
			virtualSigstore, err := ca.NewVirtualSigstore()
			if err != nil {
				t.Fatalf("NewVirtualSigstore returned error: %v", err)
			}
			return virtualSigstore, nil
		}})
		_, err := verifier.verifyEntity(context.Background(), nil, sigverify.WithArtifact(bytes.NewReader([]byte("artifact"))))
		if !apperrors.IsKind(err, apperrors.KindValidation) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("missing trusted material provider", func(t *testing.T) {
		verifier := &Verifier{expectedIssuer: GitHubActionsIssuer, expectedSAN: "san"}
		_, err := verifier.verifyEntity(context.Background(), nil, sigverify.WithArtifact(bytes.NewReader([]byte("artifact"))))
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})

	t.Run("missing expected identity configuration in verifyEntity", func(t *testing.T) {
		verifier := &Verifier{
			trustedMaterialProvider: func(context.Context) (sigroot.TrustedMaterial, error) { return nil, nil },
		}
		_, err := verifier.verifyEntity(context.Background(), nil, sigverify.WithArtifact(bytes.NewReader([]byte("artifact"))))
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})

}