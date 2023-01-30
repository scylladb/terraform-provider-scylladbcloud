package scylla

import (
	"context"
	"fmt"
	"strings"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

type CloudProvider struct {
	CloudProvider        *model.CloudProvider
	CloudProviderRegions *model.CloudProviderRegions
}

func (p *CloudProvider) RegionByID(id int64) *model.CloudProviderRegion {
	for i := range p.CloudProviderRegions.Regions {
		r := &p.CloudProviderRegions.Regions[i]

		if r.ID == id {
			return r
		}
	}
	return nil
}

func (p *CloudProvider) RegionByName(name string) *model.CloudProviderRegion {
	for i := range p.CloudProviderRegions.Regions {
		r := &p.CloudProviderRegions.Regions[i]

		if strings.EqualFold(r.ExternalID, name) {
			return r
		}
	}
	return nil
}

func (p *CloudProvider) InstanceByID(id int64) *model.CloudProviderInstance {
	for i := range p.CloudProviderRegions.Instances {
		t := &p.CloudProviderRegions.Instances[i]

		if t.ID == id {
			return t
		}
	}
	return nil
}

func (p *CloudProvider) InstanceByName(name string) *model.CloudProviderInstance {
	for i := range p.CloudProviderRegions.Instances {
		t := &p.CloudProviderRegions.Instances[i]

		if strings.EqualFold(t.ExternalID, name) {
			return t
		}
	}
	return nil
}

type Cloudmeta struct {
	AWS *CloudProvider

	CloudProviders []CloudProvider
	ScyllaVersions *model.ScyllaVersions
	ErrCodes       map[string]string
}

func BuildCloudmeta(ctx context.Context, c *Client) (*Cloudmeta, error) {
	var meta Cloudmeta

	m, err := parseCodes(codes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse error codes: %w", err)
	}

	meta.ErrCodes = m

	versions, err := c.ListScyllaVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to read scylla versions: %w", err)
	}

	meta.ScyllaVersions = versions

	providers, err := c.ListCloudProviders()
	if err != nil {
		return nil, fmt.Errorf("failed to read cloud providers: %w", err)
	}

	for i := range providers {
		p := &providers[i]

		regions, err := c.ListCloudProviderRegions(p.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to read regions for cloud provider %d: %w", p.ID, err)
		}

		meta.CloudProviders = append(meta.CloudProviders, CloudProvider{
			CloudProvider:        p,
			CloudProviderRegions: regions,
		})
	}

	aws := meta.ProviderByName("AWS")
	if aws == nil {
		return nil, fmt.Errorf("unexpected error, %q provider not found", "AWS")
	}

	meta.AWS = aws

	return &meta, nil
}

func (m *Cloudmeta) ProviderByName(name string) *CloudProvider {
	for i := range m.CloudProviders {
		p := &m.CloudProviders[i]

		if strings.EqualFold(p.CloudProvider.Name, name) {
			return p
		}
	}
	return nil
}

func (m *Cloudmeta) ProviderByID(id int64) *CloudProvider {
	for i := range m.CloudProviders {
		p := &m.CloudProviders[i]

		if p.CloudProvider.ID == id {
			return p
		}
	}
	return nil
}

func (m *Cloudmeta) DefaultVersion() *model.ScyllaVersion {
	return m.VersionByID(m.ScyllaVersions.DefaultScyllaVersionID)
}

func (m *Cloudmeta) VersionByID(id int64) *model.ScyllaVersion {
	fmt.Printf("[DEBUG] DefaultScyllaVersionID=%d\n", m.ScyllaVersions.DefaultScyllaVersionID)

	for i := range m.ScyllaVersions.ScyllaVersions {
		v := &m.ScyllaVersions.ScyllaVersions[i]

		fmt.Printf("[DEBUG] version[%d]: %+v\n", i, v)

		if v.VersionID == id {
			return v
		}
	}
	return nil
}

func (m *Cloudmeta) VersionByName(name string) *model.ScyllaVersion {
	for i := range m.ScyllaVersions.ScyllaVersions {
		v := &m.ScyllaVersions.ScyllaVersions[i]

		if strings.EqualFold(v.Version, name) {
			return v
		}
	}
	return nil
}
