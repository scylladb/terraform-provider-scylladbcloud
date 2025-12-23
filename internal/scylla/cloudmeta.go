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

func (p *CloudProvider) InstanceByIDFromInstances(id int64, instances []model.CloudProviderInstance) *model.CloudProviderInstance {
	for i := range instances {
		t := &instances[i]

		if t.ID == id {
			return t
		}
	}
	return nil
}

func (p *CloudProvider) InstanceByNameFromInstances(name string, instances []model.CloudProviderInstance) *model.CloudProviderInstance {
	for i := range instances {
		t := &instances[i]

		if strings.EqualFold(t.ExternalID, name) {
			return t
		}
	}
	return nil
}

func (p *CloudProvider) InstanceByNameAndDiskSizeFromInstances(name string, diskSize int, instances []model.CloudProviderInstance) *model.CloudProviderInstance {
	for i := range instances {
		t := &instances[i]

		if strings.EqualFold(t.ExternalID, name) && t.TotalStorage == int64(diskSize) {
			return t
		}
	}
	return nil
}

type Cloudmeta struct {
	CloudProviders []CloudProvider
	ScyllaVersions *model.ScyllaVersions
	GCPBlocks      map[string]string // region -> cidr block
}

func BuildCloudmeta(ctx context.Context, c *Client) (*Cloudmeta, error) {
	var meta Cloudmeta

	b, err := parse(blocks, blocksDelim, blocksFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cidr blocks: %w", err)
	}

	meta.GCPBlocks = b

	versions, err := c.ListScyllaVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read scylla versions: %w", err)
	}

	meta.ScyllaVersions = versions

	providers, err := c.ListCloudProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read cloud providers: %w", err)
	}

	for i := range providers {
		p := &providers[i]

		regions, err := c.ListCloudProviderRegions(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to read regions for cloud provider %d: %w", p.ID, err)
		}

		meta.CloudProviders = append(meta.CloudProviders, CloudProvider{
			CloudProvider:        p,
			CloudProviderRegions: regions,
		})
	}

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
	for i := range m.ScyllaVersions.ScyllaVersions {
		v := &m.ScyllaVersions.ScyllaVersions[i]
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
