package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
	"github.com/devr-tools/cleanr/cleanr/fsatomic"
	"gopkg.in/yaml.v3"
)

func BuildReleaseGateAttestation(report core.Report, artifact core.ReplayArtifact, rawKey string, keyID string) (core.ReleaseGateAttestation, error) {
	privateKey, err := parsePrivateKey(rawKey)
	if err != nil {
		return core.ReleaseGateAttestation{}, err
	}

	reportDigest, err := jsonDigest(report)
	if err != nil {
		return core.ReleaseGateAttestation{}, fmt.Errorf("build attestation: %w", err)
	}
	replayDigest, err := jsonDigest(artifact)
	if err != nil {
		return core.ReleaseGateAttestation{}, fmt.Errorf("build attestation: %w", err)
	}

	attestation := core.ReleaseGateAttestation{
		Version:     "v1alpha1",
		Type:        "cleanr.release_gate.attestation/v1",
		GeneratedAt: report.GeneratedAt,
		Subject: core.AttestationSubject{
			Target:               report.Name,
			ReportSHA256:         reportDigest,
			ReplayArtifactSHA256: replayDigest,
		},
		Predicate: core.AttestationPredicate{
			Passed:       report.Passed,
			FailedSuites: report.FailedSuites,
			FailedCases:  report.FailedCases,
			Metadata:     report.Metadata,
		},
	}
	if report.Metadata != nil {
		attestation.Subject.BuildID = report.Metadata.BuildID
	}
	if report.Trend != nil && !report.Trend.Baseline {
		summary := report.Trend.Summary
		attestation.Predicate.TrendSummary = &summary
	}

	unsigned, err := marshalUnsignedAttestation(attestation)
	if err != nil {
		return core.ReleaseGateAttestation{}, fmt.Errorf("build attestation: %w", err)
	}
	signature := ed25519.Sign(privateKey, unsigned)
	attestation.Signature = core.AttestationSignature{
		KeyID:     strings.TrimSpace(keyID),
		Algorithm: "ed25519",
		Value:     base64.StdEncoding.EncodeToString(signature),
	}
	return attestation, nil
}

func WriteReleaseGateAttestationFile(path string, attestation core.ReleaseGateAttestation) error {
	data, err := encodeAttestation(attestation, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Atomic: a torn attestation would carry a signature that can never verify.
	return fsatomic.WriteFile(path, append(data, '\n'), 0o644)
}

func parsePrivateKey(raw string) (ed25519.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("build attestation: empty signing key")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		if hexDecoded, hexErr := hex.DecodeString(raw); hexErr == nil {
			decoded = hexDecoded
		} else {
			return nil, fmt.Errorf("build attestation: invalid signing key encoding")
		}
	}
	switch len(decoded) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	default:
		return nil, fmt.Errorf("build attestation: signing key must be a 32-byte seed or 64-byte private key")
	}
}

func jsonDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func marshalUnsignedAttestation(attestation core.ReleaseGateAttestation) ([]byte, error) {
	unsigned := struct {
		Version     string                    `json:"version"`
		Type        string                    `json:"type"`
		GeneratedAt interface{}               `json:"generated_at"`
		Subject     core.AttestationSubject   `json:"subject"`
		Predicate   core.AttestationPredicate `json:"predicate"`
	}{
		Version:     attestation.Version,
		Type:        attestation.Type,
		GeneratedAt: attestation.GeneratedAt,
		Subject:     attestation.Subject,
		Predicate:   attestation.Predicate,
	}
	return json.Marshal(unsigned)
}

func encodeAttestation(attestation core.ReleaseGateAttestation, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(attestation)
		if err != nil {
			return nil, fmt.Errorf("encode attestation: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode attestation: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode attestation: %w", err)
		}
		return data, nil
	}
	data, err := json.MarshalIndent(attestation, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode attestation: %w", err)
	}
	return data, nil
}

func isYAMLPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}
