package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultRegistryAddr is the local OCI registry address used in the Seam lab.
// Matches the lab topology: http://10.20.0.1:5000 accessible via host bridge.
const DefaultRegistryAddr = "localhost:5000"

// RegistryClient wraps a local OCI-compatible registry for e2e test artifact
// push and pull operations. It speaks the OCI Distribution Spec (HTTP).
//
// All operations use HTTP (not HTTPS). The lab registry is plain HTTP on port 5000.
// Content type for test artifacts is application/vnd.oci.image.manifest.v1+json.
//
// Usage:
//
//	r := e2ehelpers.NewRegistryClient("localhost:5000")
//	digest, err := r.PushArtifact(ctx, "ontai-dev/test-pack", "v1.0.0", []byte("..."))
//	content, err := r.PullArtifact(ctx, "ontai-dev/test-pack", "v1.0.0")
type RegistryClient struct {
	addr   string
	client *http.Client
}

// NewRegistryClient constructs a RegistryClient for the given registry address.
// The address should be host:port without a scheme (e.g. "localhost:5000").
// If addr is empty, DefaultRegistryAddr is used.
func NewRegistryClient(addr string) *RegistryClient {
	if addr == "" {
		addr = DefaultRegistryAddr
	}
	return &RegistryClient{
		addr:   addr,
		client: &http.Client{},
	}
}

// ociManifest is the minimal OCI image manifest for a single-layer test artifact.
type ociManifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        ociDescriptor `json:"config"`
	Layers        []ociDescriptor `json:"layers"`
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

// PushArtifact pushes arbitrary content as a single-layer OCI artifact to the
// registry under the given repository and tag. Returns the manifest digest.
//
// The artifact content is stored as a blob with media type
// application/octet-stream. A minimal OCI manifest wraps it.
func (r *RegistryClient) PushArtifact(ctx context.Context, repository, tag string, content []byte) (string, error) {
	// 1. Push the blob.
	blobDigest, err := r.pushBlob(ctx, repository, content)
	if err != nil {
		return "", fmt.Errorf("e2e: registry %q: push blob for %s:%s: %w", r.addr, repository, tag, err)
	}

	// 2. Push a minimal empty config blob.
	configContent := []byte("{}")
	configDigest, err := r.pushBlob(ctx, repository, configContent)
	if err != nil {
		return "", fmt.Errorf("e2e: registry %q: push config blob for %s:%s: %w", r.addr, repository, tag, err)
	}

	// 3. Build and push the manifest.
	manifest := ociManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: ociDescriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Size:      int64(len(configContent)),
			Digest:    configDigest,
		},
		Layers: []ociDescriptor{
			{
				MediaType: "application/octet-stream",
				Size:      int64(len(content)),
				Digest:    blobDigest,
			},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("e2e: registry %q: marshal manifest: %w", r.addr, err)
	}

	manifestDigest, err := r.pushManifest(ctx, repository, tag, manifestJSON)
	if err != nil {
		return "", fmt.Errorf("e2e: registry %q: push manifest for %s:%s: %w", r.addr, repository, tag, err)
	}

	return manifestDigest, nil
}

// PullArtifact pulls the first layer blob of the OCI artifact at the given
// repository and tag. Returns the raw blob content.
func (r *RegistryClient) PullArtifact(ctx context.Context, repository, tag string) ([]byte, error) {
	// 1. Pull the manifest.
	manifestJSON, err := r.pullManifest(ctx, repository, tag)
	if err != nil {
		return nil, fmt.Errorf("e2e: registry %q: pull manifest for %s:%s: %w", r.addr, repository, tag, err)
	}

	// 2. Decode the manifest to find the first layer digest.
	var manifest ociManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return nil, fmt.Errorf("e2e: registry %q: decode manifest for %s:%s: %w", r.addr, repository, tag, err)
	}
	if len(manifest.Layers) == 0 {
		return nil, fmt.Errorf("e2e: registry %q: manifest for %s:%s has no layers", r.addr, repository, tag)
	}

	// 3. Pull the blob.
	blob, err := r.pullBlob(ctx, repository, manifest.Layers[0].Digest)
	if err != nil {
		return nil, fmt.Errorf("e2e: registry %q: pull blob for %s:%s: %w", r.addr, repository, tag, err)
	}

	return blob, nil
}

// pushBlob uploads a blob using the OCI chunked upload protocol.
// Returns "sha256:{hex}" digest.
func (r *RegistryClient) pushBlob(ctx context.Context, repository string, content []byte) (string, error) {
	sum := sha256.Sum256(content)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	// Step 1: initiate upload.
	initiateURL := fmt.Sprintf("http://%s/v2/%s/blobs/uploads/", r.addr, repository)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, initiateURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("initiate blob upload: %w", err)
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("initiate blob upload: unexpected status %d", resp.StatusCode)
	}
	uploadURL := resp.Header.Get("Location")
	if uploadURL == "" {
		return "", fmt.Errorf("initiate blob upload: no Location header")
	}
	// Registry may return relative URL.
	if !strings.HasPrefix(uploadURL, "http") {
		uploadURL = "http://" + r.addr + uploadURL
	}

	// Step 2: complete upload with blob content.
	sep := "?"
	if strings.Contains(uploadURL, "?") {
		sep = "&"
	}
	putURL := uploadURL + sep + "digest=" + digest
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(content))

	resp, err = r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("complete blob upload: %w", err)
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("complete blob upload: unexpected status %d", resp.StatusCode)
	}

	return digest, nil
}

// pullBlob retrieves a blob by digest.
func (r *RegistryClient) pullBlob(ctx context.Context, repository, digest string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/v2/%s/blobs/%s", r.addr, repository, digest)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pull blob %s: status %d", digest, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// pushManifest uploads a manifest under the given tag and returns its digest.
func (r *RegistryClient) pushManifest(ctx context.Context, repository, tag string, manifestJSON []byte) (string, error) {
	url := fmt.Sprintf("http://%s/v2/%s/manifests/%s", r.addr, repository, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(manifestJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	req.ContentLength = int64(len(manifestJSON))

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("push manifest: status %d", resp.StatusCode)
	}

	sum := sha256.Sum256(manifestJSON)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// pullManifest retrieves the manifest JSON for the given repository and tag.
func (r *RegistryClient) pullManifest(ctx context.Context, repository, tag string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/v2/%s/manifests/%s", r.addr, repository, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pull manifest %s:%s: status %d", repository, tag, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
