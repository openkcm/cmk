package integrationutils

import (
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/cmk-core/internal/config"
)

var DB = config.Database{
	Host: commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  "localhost",
	},
	User: commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  "admin",
	},
	Secret: commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  "password",
	},
	Name: "mydb",
	Port: "5432",
}

var ReplicaDB = []config.Database{
	{
		Host: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "localhost",
		},
		User: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "admin",
		},
		Secret: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "password",
		},
		Name: "mydb",
		Port: "5433",
	},
}

var MessageService = config.Redis{
	Host: commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  "localhost",
	},
	ACL: config.RedisACL{
		Enabled: true,
		Password: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "secret",
		},
		Username: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "default",
		},
	},
	Port: "6379",
	SecretRef: commoncfg.SecretRef{
		Type: commoncfg.InsecureSecretType,
	},
}
