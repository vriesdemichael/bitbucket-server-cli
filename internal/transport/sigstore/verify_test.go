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
}