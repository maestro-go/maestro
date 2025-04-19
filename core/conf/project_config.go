package conf

type sslConfig struct {
	SSLMode     string `yaml:"sslmode" default:"disable"`
	SSLRootCert string `yaml:"sslrootcert,omitempty"`
}

type MigrationConfig struct {
	Locations        []string `yaml:"locations" default:"[\"./migrations\"]"`
	Validate         bool     `yaml:"validate" default:"true"`
	Down             bool     `yaml:"down,omitempty"`
	InTransaction    bool     `yaml:"in-transaction" default:"true"`
	Destination      *uint16  `yaml:"destination,omitempty"`
	Force            bool     `yaml:"force" default:"false"`
	UseRepeatable    bool     `yaml:"use-repeatable" default:"true"`
	UseBefore        bool     `yaml:"use-before" default:"true"`
	UseAfter         bool     `yaml:"use-after" default:"true"`
	UseBeforeEach    bool     `yaml:"use-before-each" default:"true"`
	UseAfterEach     bool     `yaml:"use-after-each" default:"true"`
	UseBeforeVersion bool     `yaml:"use-before-version" default:"true"`
	UseAfterVersion  bool     `yaml:"use-after-version" default:"true"`
}

type ProjectConfig struct {
	Driver       string `yaml:"driver" default:"postgres"`
	Host         string `yaml:"host" default:"localhost"`
	Port         uint16 `yaml:"port" default:"5432"`
	Database     string `yaml:"database" default:"postgres"`
	User         string `yaml:"user" default:"postgres"`
	Password     string `yaml:"password" default:"postgres"`
	Schema       string `yaml:"schema" default:"public"`
	HistoryTable string `yaml:"history-table" default:"schema_history"`

	SSL sslConfig `yaml:"ssl"`

	Migration MigrationConfig `yaml:"migrations"`
}
