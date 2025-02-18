package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Client handles produce, consume and ACL-related operations
type Client struct {
	config *Config
	logger *zap.Logger
}

// NewClient creates a new ACLManager
func NewClient(config *Config, logger *zap.Logger) *Client {
	return &Client{
		config: config,
		logger: logger,
	}
}

// newClusterAdmin creates a new sarama.ClusterAdmin
func (c *Client) newClusterAdmin() (sarama.ClusterAdmin, error) {
	saramaConfig, err := c.config.ToSaramaConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create sarama config: %w", err)
	}

	admin, err := sarama.NewClusterAdmin(c.config.GetBrokers(), saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}

	return admin, nil
}

// CreateACL creates a new ACL
func (c *Client) CreateACL(resource sarama.Resource, acl sarama.Acl) error {
	admin, err := c.newClusterAdmin()
	if err != nil {
		return fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer admin.Close()

	err = admin.CreateACL(resource, acl)
	if err != nil {
		return fmt.Errorf("failed to create ACL: %w", err)
	}

	c.logger.Info("ACL added",
		zap.Any("resource", resource),
		zap.Any("acl", acl))
	return nil
}

// ListAcls lists ACLs based on the provided filter
func (c *Client) ListAcls(filter sarama.AclFilter) ([]sarama.ResourceAcls, error) {
	admin, err := c.newClusterAdmin()
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer admin.Close()

	acls, err := admin.ListAcls(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list ACLs: %w", err)
	}

	for _, acl := range acls {
		c.logger.Info("ACL found",
			zap.Any("resource", acl.Resource),
			zap.Any("acls", acl.Acls))
	}

	return acls, nil
}

// DeleteACL deletes ACLs based on the provided filter
func (c *Client) DeleteACL(filter sarama.AclFilter) ([]sarama.MatchingAcl, error) {
	admin, err := c.newClusterAdmin()
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer admin.Close()

	matchingACLs, err := admin.DeleteACL(filter, false)
	if err != nil {
		return nil, fmt.Errorf("failed to delete ACL: %w", err)
	}

	for _, matchingACL := range matchingACLs {
		c.logger.Info("Deleted ACL",
			zap.Any("matchingACL", matchingACL))
	}

	return matchingACLs, nil
}
