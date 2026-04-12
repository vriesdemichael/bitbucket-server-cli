package sigstore

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	sigroot "github.com/sigstore/sigstore-go/pkg/root"
	sigverify "github.com/sigstore/sigstore-go/pkg/verify"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

const (
	GitHubActionsIssuer = "https://token.actions.githubusercontent.com"
	ReleaseWorkflowPath = ".github/workflows/release.yml"
	MainBranchRef       = "refs/heads/main"
)

type TrustedMaterialProvider func(context.Context) (sigroot.TrustedMaterial, error)

type Options struct {
	ExpectedIssuer          string
	ExpectedSAN             string
	TrustedMaterialProvider TrustedMaterialProvider
	VerifierOptions         []sigverify.VerifierOption
}

type Verification struct {
	CertificateIdentity           string
	CertificateOIDCIssuer         string
	TransparencyLogEntriesVerified int
	VerifiedTimestampCount        int
}

type Verifier struct {
	expectedIssuer          string
	expectedSAN             string
	trustedMaterialProvider TrustedMaterialProvider
	verifierOptions         []sigverify.VerifierOption
}

func NewGitHubReleaseVerifier(owner, repo string) *Verifier {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)

	return NewVerifier(Options{
		ExpectedIssuer: GitHubActionsIssuer,
		ExpectedSAN:    fmt.Sprintf("https://github.com/%s/%s/%s@%s", owner, repo, ReleaseWorkflowPath, MainBranchRef),
	})
}

func NewVerifier(options Options) *Verifier {
	provider := options.TrustedMaterialProvider
	if provider == nil {
		provider = func(context.Context) (sigroot.TrustedMaterial, error) {
			trustedRoot, err := sigroot.FetchTrustedRoot()
			if err != nil {
				return nil, apperrors.New(apperrors.KindTransient, "failed to load Sigstore trusted roots", err)
			}
			return trustedRoot, nil
		}
	}

	verifierOptions := options.VerifierOptions
	if len(verifierOptions) == 0 {
		verifierOptions = []sigverify.VerifierOption{
			sigverify.WithSignedCertificateTimestamps(1),
			sigverify.WithTransparencyLog(1),
			sigverify.WithObserverTimestamps(1),
		}
	}

	return &Verifier{
		expectedIssuer:          strings.TrimSpace(options.ExpectedIssuer),
		expectedSAN:             strings.TrimSpace(options.ExpectedSAN),
		trustedMaterialProvider: provider,
		verifierOptions:         verifierOptions,
	}
}

func (verifier *Verifier) VerifyBlob(ctx context.Context, artifact, bundleJSON []byte) (Verification, error) {
	if verifier == nil || verifier.trustedMaterialProvider == nil {
		return Verification{}, apperrors.New(apperrors.KindInternal, "sigstore verifier is not configured", nil)
	}
	if len(artifact) == 0 {
		return Verification{}, apperrors.New(apperrors.KindValidation, "artifact content is required for Sigstore verification", nil)
	}
	if len(bundleJSON) == 0 {
		return Verification{}, apperrors.New(apperrors.KindValidation, "Sigstore bundle content is required", nil)
	}
	if verifier.expectedIssuer == "" || verifier.expectedSAN == "" {
		return Verification{}, apperrors.New(apperrors.KindInternal, "sigstore verifier identity is not configured", nil)
	}

	var entity bundle.Bundle
	if err := entity.UnmarshalJSON(bundleJSON); err != nil {
		return Verification{}, apperrors.New(apperrors.KindPermanent, "release signature bundle is invalid", err)
	}

	return verifier.verifyEntity(ctx, &entity, sigverify.WithArtifact(bytes.NewReader(artifact)))
}

func (verifier *Verifier) verifyEntity(ctx context.Context, entity sigverify.SignedEntity, artifactPolicy sigverify.ArtifactPolicyOption, policyOptions ...sigverify.PolicyOption) (Verification, error) {
	if verifier == nil || verifier.trustedMaterialProvider == nil {
		return Verification{}, apperrors.New(apperrors.KindInternal, "sigstore verifier is not configured", nil)
	}
	if verifier.expectedIssuer == "" || verifier.expectedSAN == "" {
		return Verification{}, apperrors.New(apperrors.KindInternal, "sigstore verifier identity is not configured", nil)
	}
	if entity == nil {
		return Verification{}, apperrors.New(apperrors.KindValidation, "Sigstore signed entity is required", nil)
	}

	trustedMaterial, err := verifier.trustedMaterialProvider(ctx)
	if err != nil {
		return Verification{}, err
	}

	signedEntityVerifier, err := sigverify.NewSignedEntityVerifier(trustedMaterial, verifier.verifierOptions...)
	if err != nil {
		return Verification{}, apperrors.New(apperrors.KindInternal, "failed to initialize Sigstore verifier", err)
	}

	certificateIdentity, err := sigverify.NewShortCertificateIdentity(verifier.expectedIssuer, "", verifier.expectedSAN, "")
	if err != nil {
		return Verification{}, apperrors.New(apperrors.KindInternal, "failed to configure Sigstore certificate identity", err)
	}

	policyOptions = append(policyOptions, sigverify.WithCertificateIdentity(certificateIdentity))
	result, err := signedEntityVerifier.Verify(entity, sigverify.NewPolicy(artifactPolicy, policyOptions...))
	if err != nil {
		return Verification{}, apperrors.New(apperrors.KindPermanent, "release signature verification failed", err)
	}
	if result == nil || result.Signature == nil || result.Signature.Certificate == nil {
		return Verification{}, apperrors.New(apperrors.KindPermanent, "release signature verification did not return a signing certificate", nil)
	}

	transparencyLogEntriesVerified := 0
	for _, timestamp := range result.VerifiedTimestamps {
		if strings.EqualFold(timestamp.Type, "tlog") {
			transparencyLogEntriesVerified++
		}
	}

	return Verification{
		CertificateIdentity:            strings.TrimSpace(result.Signature.Certificate.SubjectAlternativeName),
		CertificateOIDCIssuer:          strings.TrimSpace(result.Signature.Certificate.Extensions.Issuer),
		TransparencyLogEntriesVerified: transparencyLogEntriesVerified,
		VerifiedTimestampCount:         len(result.VerifiedTimestamps),
	}, nil
}