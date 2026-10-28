module github.com/medici-finance/assay/statusgen

go 1.27.0

require (
	github.com/medici-finance/assay/deskcore v0.0.0-00010101000000-000000000000
	github.com/medici-finance/assay/forgeread v0.0.0-00010101000000-000000000000
	github.com/medici-finance/assay/gitquiet v0.0.0-00010101000000-000000000000
	github.com/medici-finance/assay/perfrec v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.10.2
	github.com/spf13/viper v1.20.1
	golang.org/x/sys v0.46.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/medici-finance/assay/forgeread => ../forgeread

replace github.com/medici-finance/assay/perfrec => ../perfrec

replace github.com/medici-finance/assay/gitquiet => ../gitquiet

replace github.com/medici-finance/assay/deskcore => ../deskcore
