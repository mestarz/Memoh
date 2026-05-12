package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultConfigPath       = "config.toml"
	DefaultHTTPAddr         = ":8080"
	DefaultNamespace        = "default"
	DefaultDataRoot         = "data"
	DefaultDataMount        = "/data"
	DefaultCNIBinaryDir     = "/opt/cni/bin"
	DefaultCNIConfigDir     = "/etc/cni/net.d"
	DefaultJWTExpiresIn     = "24h"
	DefaultDatabaseDriver   = "postgres"
	DefaultPGHost           = "127.0.0.1"
	DefaultPGPort           = 5432
	DefaultPGUser           = "postgres"
	DefaultPGDatabase       = "memoh"
	DefaultPGSSLMode        = "disable"
	DefaultQdrantURL        = "http://127.0.0.1:6334"
	DefaultQdrantCollection = "memory"
	DefaultRuntimeDir       = "/opt/memoh/runtime"
	DefaultBaseImage        = "debian:bookworm-slim"
	DefaultTimezone         = "UTC"

	ImagePullPolicyIfNotPresent = "if_not_present"
	ImagePullPolicyAlways       = "always"
	ImagePullPolicyNever        = "never"
)

type Config struct {
	Log         LogConfig         `toml:"log"`
	Server      ServerConfig      `toml:"server"`
	Admin       AdminConfig       `toml:"admin"`
	Auth        AuthConfig        `toml:"auth"`
	Timezone    string            `toml:"timezone"`
	Database    DatabaseConfig    `toml:"database"`
	Container   ContainerConfig   `toml:"container"`
	Docker      DockerConfig      `toml:"docker"`
	Local       LocalConfig       `toml:"local"`
	Workspace   WorkspaceConfig   `toml:"workspace"`
	Display     DisplayConfig     `toml:"display"`
	Postgres    PostgresConfig    `toml:"postgres"`
	Qdrant      QdrantConfig      `toml:"qdrant"`
	Sparse      SparseConfig      `toml:"sparse"`
	Registry    RegistryConfig    `toml:"registry"`
	Supermarket SupermarketConfig `toml:"supermarket"`
}

// DisplayConfig configures the bot workspace remote desktop pipeline.
type DisplayConfig struct {
	WebRTC DisplayWebRTCConfig `toml:"webrtc"`
}

// DisplayWebRTCConfig tunes the WebRTC transport that streams the bot's
// workspace desktop to the browser. The defaults are tuned for single-host
// usage; LAN deployments should usually set udp_port_min/max (and optionally
// tcp_port) to predictable values so firewall rules can be opened, and may
// need to set nat_ips when the server is reached through a hostname/NAT
// the auto-detection cannot infer.
type DisplayWebRTCConfig struct {
	// UDPPortMin / UDPPortMax confine the ephemeral UDP port range used for
	// ICE host candidates. When zero the OS picks any free port.
	UDPPortMin uint16 `toml:"udp_port_min"`
	UDPPortMax uint16 `toml:"udp_port_max"`
	// TCPPort enables an ICE-TCP fallback listener bound to 0.0.0.0 on the
	// configured port. When 0 the fallback is disabled.
	TCPPort uint16 `toml:"tcp_port"`
	// NATIPs are extra public/LAN IPs to advertise as host candidates. They
	// are unioned with addresses inferred from the request and (when
	// auto_nat_ips is enabled) the host's own non-loopback interfaces.
	NATIPs []string `toml:"nat_ips"`
	// AutoNATIPs controls whether the server advertises every non-loopback
	// IPv4 it finds on local network interfaces. Enabled by default so that
	// LAN clients can establish a host candidate without extra configuration.
	AutoNATIPs *bool `toml:"auto_nat_ips"`
	// AutoNATIncludeCIDRs optionally narrows auto_nat_ips to only addresses
	// inside one of the listed CIDRs. When empty (default) every non-loopback
	// non-link-local IP is advertised. Useful to skip docker/veth bridges,
	// e.g. ["192.168.0.0/16", "10.8.0.0/16", "100.64.0.0/10"] keeps LAN, VPN
	// and Tailscale ranges while dropping 172.16/12 docker bridges.
	AutoNATIncludeCIDRs []string `toml:"auto_nat_include_cidrs"`
	// STUNServers configures public STUN servers for ICE gathering. Unset
	// by default: pure-LAN deployments do not need STUN, and we don't want
	// to leak traffic to third parties without explicit opt-in.
	STUNServers []string `toml:"stun_servers"`
}

// AutoNATEnabled returns whether host-IP auto-discovery is active.
// When AutoNATIPs is nil, the default is true.
func (c DisplayWebRTCConfig) AutoNATEnabled() bool {
	if c.AutoNATIPs == nil {
		return true
	}
	return *c.AutoNATIPs
}

type LogConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

type ServerConfig struct {
	Addr string `toml:"addr"`
}

type AdminConfig struct {
	Username string `toml:"username"`
	Password string `toml:"password" json:"-"`
	Email    string `toml:"email"`
}

type AuthConfig struct {
	JWTSecret    string `toml:"jwt_secret"    json:"-"`
	JWTExpiresIn string `toml:"jwt_expires_in"`
}

type DatabaseConfig struct {
	Driver string `toml:"driver"`
}

func (c DatabaseConfig) DriverOrDefault() string {
	driver := strings.TrimSpace(strings.ToLower(c.Driver))
	if driver == "" {
		return DefaultDatabaseDriver
	}
	return driver
}

type ContainerConfig struct {
	Backend string `toml:"backend"`
	WorkspaceConfig
}

type DockerConfig struct {
	Host string `toml:"host"`
}

type LocalConfig struct {
	Enabled                bool   `toml:"enabled"`
	DefaultWorkspaceParent string `toml:"default_workspace_parent"`
	MetadataRoot           string `toml:"metadata_root"`
	AllowAbsolutePaths     bool   `toml:"allow_absolute_paths"`
}

func (c LocalConfig) WorkspaceParent() string {
	if strings.TrimSpace(c.DefaultWorkspaceParent) != "" {
		return expandHome(strings.TrimSpace(c.DefaultWorkspaceParent))
	}
	return filepath.Join(homeDirOrDot(), ".memoh", "workspaces")
}

func (c LocalConfig) MetadataPath(dataRoot string) string {
	if strings.TrimSpace(c.MetadataRoot) != "" {
		return expandHome(strings.TrimSpace(c.MetadataRoot))
	}
	root := strings.TrimSpace(dataRoot)
	if root == "" {
		root = DefaultDataRoot
	}
	return filepath.Join(root, "local", "containers")
}

type WorkspaceConfig struct {
	Registry        string `toml:"registry"`
	DefaultImage    string `toml:"default_image"`
	ImagePullPolicy string `toml:"image_pull_policy"`
	Snapshotter     string `toml:"snapshotter"`
	DataRoot        string `toml:"data_root"`
	CNIBinaryDir    string `toml:"cni_bin_dir"`
	CNIConfigDir    string `toml:"cni_conf_dir"`
	RuntimeDir      string `toml:"runtime_dir"`
}

// ImageRef returns the fully qualified image reference for the base image,
// prepending the registry mirror when configured and normalizing for the
// container runtime.
func (c WorkspaceConfig) ImageRef() string {
	img := c.DefaultImage
	if img == "" {
		img = DefaultBaseImage
	}
	if c.Registry != "" {
		return c.Registry + "/" + img
	}
	return NormalizeImageRef(img)
}

// RuntimePath returns the path to the workspace runtime directory.
func (c WorkspaceConfig) RuntimePath() string {
	if c.RuntimeDir != "" {
		return c.RuntimeDir
	}
	return DefaultRuntimeDir
}

func (c WorkspaceConfig) EffectiveImagePullPolicy() string {
	switch strings.TrimSpace(strings.ToLower(c.ImagePullPolicy)) {
	case ImagePullPolicyAlways:
		return ImagePullPolicyAlways
	case ImagePullPolicyNever:
		return ImagePullPolicyNever
	case ImagePullPolicyIfNotPresent, "":
		return ImagePullPolicyIfNotPresent
	default:
		return ImagePullPolicyIfNotPresent
	}
}

func expandHome(path string) string {
	if path == "~" {
		return homeDirOrDot()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDirOrDot(), path[2:])
	}
	return path
}

func homeDirOrDot() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	return "."
}

// NormalizeImageRef ensures an image reference is fully qualified.
func NormalizeImageRef(ref string) string {
	firstSlash := strings.Index(ref, "/")
	if firstSlash == -1 {
		return "docker.io/library/" + ref
	}
	firstSegment := ref[:firstSlash]
	if strings.Contains(firstSegment, ".") || strings.Contains(firstSegment, ":") || firstSegment == "localhost" {
		return ref
	}
	return "docker.io/" + ref
}

type PostgresConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password" json:"-"`
	Database string `toml:"database"`
	SSLMode  string `toml:"sslmode"`
}

type QdrantConfig struct {
	BaseURL        string `toml:"base_url"`
	APIKey         string `toml:"api_key" json:"-"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
}

type SparseConfig struct {
	BaseURL string `toml:"base_url"`
}

const DefaultProvidersDir = "conf/providers"

type RegistryConfig struct {
	ProvidersDir     string `toml:"providers_dir"`
	DisableModelSync bool   `toml:"disable_model_sync"`
}

// ProvidersPath returns the configured providers directory or the default.
func (c RegistryConfig) ProvidersPath() string {
	if c.ProvidersDir != "" {
		return c.ProvidersDir
	}
	return DefaultProvidersDir
}

const DefaultSupermarketBaseURL = "https://supermarket.memoh.ai"

type SupermarketConfig struct {
	BaseURL string `toml:"base_url"`
}

func (c SupermarketConfig) GetBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultSupermarketBaseURL
}

func Load(path string) (Config, error) {
	defaultWorkspace := WorkspaceConfig{
		DefaultImage: DefaultBaseImage,
		DataRoot:     DefaultDataRoot,
		CNIBinaryDir: DefaultCNIBinaryDir,
		CNIConfigDir: DefaultCNIConfigDir,
	}
	cfg := Config{
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Server: ServerConfig{
			Addr: DefaultHTTPAddr,
		},
		Admin: AdminConfig{
			Username: "admin",
			Password: "change-your-password-here",
			Email:    "you@example.com",
		},
		Auth: AuthConfig{
			JWTExpiresIn: DefaultJWTExpiresIn,
		},
		Timezone: DefaultTimezone,
		Database: DatabaseConfig{
			Driver: DefaultDatabaseDriver,
		},
		Container: ContainerConfig{
			Backend:         "",
			WorkspaceConfig: defaultWorkspace,
		},
		Workspace: defaultWorkspace,
		Postgres: PostgresConfig{
			Host:     DefaultPGHost,
			Port:     DefaultPGPort,
			User:     DefaultPGUser,
			Database: DefaultPGDatabase,
			SSLMode:  DefaultPGSSLMode,
		},
	}

	if path == "" {
		path = DefaultConfigPath
	}
	path = filepath.Clean(path)

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	//nolint:gosec // config path is intentionally user-configurable
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	var raw struct {
		Container map[string]any `toml:"container"`
		Workspace map[string]any `toml:"workspace"`
		MCP       map[string]any `toml:"mcp"`
	}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return cfg, err
	}
	if raw.MCP != nil {
		if raw.Workspace != nil {
			return cfg, errors.New("config uses both [mcp] and [workspace]; remove [mcp] and move workspace fields into [container]")
		}
		return cfg, errors.New("config section [mcp] has been replaced by workspace fields in [container]; update your config.toml and restart")
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, err
	}
	if raw.Workspace != nil && containerHasWorkspaceFields(raw.Container) {
		return cfg, errors.New("config uses workspace fields in both [container] and [workspace]; move workspace fields into [container] and remove [workspace]")
	}
	if raw.Workspace != nil {
		cfg.Container.WorkspaceConfig = cfg.Workspace
	} else {
		cfg.Workspace = cfg.Container.WorkspaceConfig
	}

	return cfg, nil
}

func containerHasWorkspaceFields(values map[string]any) bool {
	for _, key := range []string{
		"registry",
		"default_image",
		"image_pull_policy",
		"snapshotter",
		"data_root",
		"cni_bin_dir",
		"cni_conf_dir",
		"runtime_dir",
	} {
		if _, ok := values[key]; ok {
			return true
		}
	}
	return false
}
