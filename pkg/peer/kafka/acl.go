package kafka

import (
	"github.com/IBM/sarama"
)

func CreateACL(config KafkaConfig, resource sarama.Resource, acl sarama.Acl) error {
	conf, brokers, err := InitConfig(config)
	if err != nil {
		return err
	}

	admin, err := sarama.NewClusterAdmin(brokers, conf)
	if err != nil {
		return err
	}
	defer admin.Close()

	err = admin.CreateACL(resource, acl)
	if err != nil {
		return err
	}

	logger.Printf("ACL added: %+v, %+v\n", resource, acl)
	return nil
}

func ListAcls(config KafkaConfig, filter sarama.AclFilter) ([]sarama.ResourceAcls, error) {
	conf, brokers, err := InitConfig(config)
	if err != nil {
		return nil, err
	}

	admin, err := sarama.NewClusterAdmin(brokers, conf)
	if err != nil {
		return nil, err
	}
	defer admin.Close()

	acls, err := admin.ListAcls(filter)
	if err != nil {
		return nil, err
	}

	for _, acl := range acls {
		logger.Printf("ACL: %+v\n", acl)
	}

	return acls, nil
}

func DeleteACL(config KafkaConfig, filter sarama.AclFilter) ([]sarama.MatchingAcl, error) {
	conf, brokers, err := InitConfig(config)
	if err != nil {
		return nil, err
	}

	admin, err := sarama.NewClusterAdmin(brokers, conf)
	if err != nil {
		return nil, err
	}
	defer admin.Close()

	matchingACLs, err := admin.DeleteACL(filter, false)
	if err != nil {
		return nil, err
	}

	for _, matchingACL := range matchingACLs {
		logger.Printf("Deleted ACL: %+v\n", matchingACL)
	}

	return matchingACLs, nil
}
