package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/version"
	toolkitweb "github.com/prometheus/exporter-toolkit/web"
	toolkitflags "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Web       WebConfig       `yaml:"web"`
	Collector CollectorConfig `yaml:"collector"`
	PBS       PBSConfig       `yaml:"pbs"`
}

type WebConfig struct {
	TelemetryPath string `yaml:"telemetry_path"`
}

type CollectorConfig struct {
	Interval                    time.Duration `yaml:"-"`
	Timeout                     time.Duration `yaml:"-"`
	IncludeUserMetrics          bool          `yaml:"include_user_metrics"`
	IncludeJobInspectionMetrics bool          `yaml:"include_job_inspection_metrics"`
}

type PBSConfig struct {
	BinaryDir string `yaml:"binary_dir"`
}

type Parsed struct {
	Runtime    Config
	Web        *toolkitweb.FlagConfig
	Log        *promslog.Config
	ConfigFile string
}

type fileConfig struct {
	Web struct {
		TelemetryPath string `yaml:"telemetry_path"`
	} `yaml:"web"`
	Collector struct {
		Interval                    duration `yaml:"interval"`
		Timeout                     duration `yaml:"timeout"`
		IncludeUserMetrics          bool     `yaml:"include_user_metrics"`
		IncludeJobInspectionMetrics bool     `yaml:"include_job_inspection_metrics"`
	} `yaml:"collector"`
	PBS struct {
		BinaryDir string `yaml:"binary_dir"`
	} `yaml:"pbs"`
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a scalar")
	}
	value, err := time.ParseDuration(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	d.Duration = value
	return nil
}

func Default() Config {
	return Config{
		Web: WebConfig{
			TelemetryPath: "/metrics",
		},
		Collector: CollectorConfig{
			Interval:                    time.Minute,
			Timeout:                     15 * time.Second,
			IncludeUserMetrics:          false,
			IncludeJobInspectionMetrics: false,
		},
		PBS: PBSConfig{
			BinaryDir: "",
		},
	}
}

func Parse(args []string) (*Parsed, error) {
	cfg := Default()
	configFile := extractConfigFile(args)
	if configFile != "" {
		loaded, err := LoadFile(configFile)
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}

	app := kingpin.New("pbs-exporter", "Prometheus exporter for PBS clusters.")
	app.HelpFlag.Short('h')
	app.Version(version.Print("pbs_exporter"))

	flagConfigFile := app.Flag("config.file", "Path to the exporter runtime configuration file.").Default(configFile).String()
	telemetryPath := app.Flag("web.telemetry-path", "Path under which to expose metrics.").Default(cfg.Web.TelemetryPath).String()
	interval := app.Flag("collector.interval", "How often to refresh the cached PBS snapshot.").Default(cfg.Collector.Interval.String()).Duration()
	timeout := app.Flag("collector.timeout", "Per-command timeout for PBS CLI invocations.").Default(cfg.Collector.Timeout.String()).Duration()
	includeUserMetrics := app.Flag("collector.include-user-metrics", "Expose user-labeled job metrics.").Default(strconv.FormatBool(cfg.Collector.IncludeUserMetrics)).Bool()
	includeJobInspectionMetrics := app.Flag("collector.include-job-inspection-metrics", "Expose per-job inspection metrics from qstat JSON output.").Default(strconv.FormatBool(cfg.Collector.IncludeJobInspectionMetrics)).Bool()
	binaryDir := app.Flag("pbs.binary-dir", "Directory containing PBS CLI binaries. Empty uses PATH.").Default(cfg.PBS.BinaryDir).String()

	logLevel := promslog.NewLevel()
	_ = logLevel.Set("info")
	logFormat := promslog.NewFormat()
	_ = logFormat.Set("logfmt")
	app.Flag("log.level", "Only log messages with the given severity or above.").Default("info").SetValue(logLevel)
	app.Flag("log.format", "Output format of log messages.").Default("logfmt").SetValue(logFormat)

	webFlags := toolkitflags.AddFlags(app, ":9785")

	if _, err := app.Parse(args); err != nil {
		return nil, err
	}

	return &Parsed{
		Runtime: Config{
			Web: WebConfig{
				TelemetryPath: *telemetryPath,
			},
			Collector: CollectorConfig{
				Interval:                    *interval,
				Timeout:                     *timeout,
				IncludeUserMetrics:          *includeUserMetrics,
				IncludeJobInspectionMetrics: *includeJobInspectionMetrics,
			},
			PBS: PBSConfig{
				BinaryDir: *binaryDir,
			},
		},
		Web:        webFlags,
		Log:        &promslog.Config{Level: logLevel, Format: logFormat},
		ConfigFile: *flagConfigFile,
	}, nil
}

func LoadFile(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	var fileCfg fileConfig
	if err := yaml.Unmarshal(content, &fileCfg); err != nil {
		return Config{}, fmt.Errorf("parse config file %q: %w", path, err)
	}

	if fileCfg.Web.TelemetryPath != "" {
		cfg.Web.TelemetryPath = fileCfg.Web.TelemetryPath
	}
	if fileCfg.Collector.Interval.Duration > 0 {
		cfg.Collector.Interval = fileCfg.Collector.Interval.Duration
	}
	if fileCfg.Collector.Timeout.Duration > 0 {
		cfg.Collector.Timeout = fileCfg.Collector.Timeout.Duration
	}
	cfg.Collector.IncludeUserMetrics = fileCfg.Collector.IncludeUserMetrics
	cfg.Collector.IncludeJobInspectionMetrics = fileCfg.Collector.IncludeJobInspectionMetrics
	if fileCfg.PBS.BinaryDir != "" {
		cfg.PBS.BinaryDir = fileCfg.PBS.BinaryDir
	}

	return cfg, nil
}

func extractConfigFile(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config.file" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(arg, "--config.file="):
			return strings.TrimPrefix(arg, "--config.file=")
		}
	}
	return ""
}
