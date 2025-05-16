package resource

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/ikafly144/sabalauncher/pkg/osinfo"
)

type ClientManifest struct {
	Arguments              Arguments   `json:"arguments"`
	AssetIndex             AssetIndex  `json:"assetIndex"`
	Assets                 string      `json:"assets"`
	ComplianceLevel        int         `json:"complianceLevel"`
	Downloads              Downloads   `json:"downloads"`
	ID                     string      `json:"id"`
	JavaVersion            JavaVersion `json:"javaVersion"`
	Libraries              []Library   `json:"libraries"`
	Logging                Logging     `json:"logging"`
	MainClass              string      `json:"mainClass"`
	MinimumLauncherVersion int         `json:"minimumLauncherVersion"`
	ReleaseTime            time.Time   `json:"releaseTime"`
	Time                   time.Time   `json:"time"`
	Type                   string      `json:"type"`
	InheritsFrom           string      `json:"inheritsFrom,omitempty"`
}

func (c ClientManifest) InheritsMerge(other *ClientManifest) (*ClientManifest, error) {
	if other == nil {
		return nil, errors.New("other manifest is nil")
	}
	if c.InheritsFrom != "" {
		return nil, errors.New("cannot inherit from another manifest")
	}
	if other.InheritsFrom == "" {
		return nil, errors.New("other manifest does not inherit from another manifest")
	}
	if c.ID != other.InheritsFrom {
		return nil, errors.New("other manifest does not inherit from this manifest")
	}
	n := ClientManifest{
		Arguments:              Arguments{},
		AssetIndex:             c.AssetIndex,
		Assets:                 c.Assets,
		ComplianceLevel:        c.ComplianceLevel,
		Downloads:              c.Downloads,
		ID:                     c.ID,
		JavaVersion:            c.JavaVersion,
		Libraries:              []Library{},
		Logging:                c.Logging,
		MainClass:              c.MainClass,
		MinimumLauncherVersion: c.MinimumLauncherVersion,
		ReleaseTime:            c.ReleaseTime,
		Time:                   c.Time,
		Type:                   c.Type,
		InheritsFrom:           c.InheritsFrom,
	}

	// Merge the other manifest into this one
	n.Arguments.Game = append(n.Arguments.Game, c.Arguments.Game...)
	n.Arguments.Game = append(n.Arguments.Game, other.Arguments.Game...)
	n.Arguments.Jvm = append(n.Arguments.Jvm, c.Arguments.Jvm...)
	n.Arguments.Jvm = append(n.Arguments.Jvm, other.Arguments.Jvm...)
	n.Libraries = append(n.Libraries, c.Libraries...)
	n.Libraries = append(n.Libraries, other.Libraries...)
	n.MainClass = other.MainClass
	n.ID = other.ID
	n.Type = other.Type
	n.InheritsFrom = other.InheritsFrom
	return &n, nil
}

type Arguments struct {
	Game []GameArgument `json:"game"`
	Jvm  []JvmArgument  `json:"jvm"`
}

func (a *Arguments) UnmarshalJSON(data []byte) error {
	type unmarshal struct {
		Game []GameArgumentUnmarshal `json:"game"`
		Jvm  []JvmArgumentUnmarshal  `json:"jvm"`
	}
	var raw unmarshal
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.Game = make([]GameArgument, len(raw.Game))
	for i, arg := range raw.Game {
		a.Game[i] = arg.GameArgument
	}
	a.Jvm = make([]JvmArgument, len(raw.Jvm))
	for i, arg := range raw.Jvm {
		a.Jvm[i] = arg.JvmArgument
	}
	return nil
}

type RuleAction string

const (
	RuleActionAllow RuleAction = "allow"
	RuleActionDeny  RuleAction = "deny"
)

func (a RuleAction) Allowed() bool {
	switch a {
	case RuleActionAllow:
		return true
	case RuleActionDeny:
		return false
	default:
		return false
	}
}

type ArgumentValue []string

func (a ArgumentValue) MarshalJSON() ([]byte, error) {
	if len(a) == 1 {
		return json.Marshal(a[0])
	}
	return json.Marshal([]string(a))
}

func (a *ArgumentValue) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch raw.(type) {
	case string:
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*a = append(*a, str)
	case []any:
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*a = append(*a, arr...)
	default:
		return errors.New("invalid argument value type")
	}

	return nil
}

type GameArgument interface {
	MarshalJSON() ([]byte, error)
	gameArgument()
}

type GameArgumentString string

func (g GameArgumentString) String() string {
	return string(g)
}

func (g GameArgumentString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(g))
}

func (g GameArgumentString) gameArgument() {}

type GameArgumentRule struct {
	Rules []GameArgumentRuleType `json:"rules"`
	Value ArgumentValue          `json:"value"`
}

func (g GameArgumentRule) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Rules []GameArgumentRuleType `json:"rules"`
		Value ArgumentValue          `json:"value"`
	}{
		Rules: g.Rules,
		Value: g.Value,
	})
}

func (g GameArgumentRule) gameArgument() {}

type GameArgumentRuleType struct {
	Action   RuleAction                   `json:"action"`
	Features GameArgumentRuleTypeFeatures `json:"features"`
}

type GameArgumentRuleTypeFeatures struct {
	IsDemoUser              bool `json:"is_demo_user"`
	HasCustomResolution     bool `json:"has_custom_resolution"`
	HasQuickPlaysSupport    bool `json:"has_quick_plays_support"`
	IsQuickPlaySingleplayer bool `json:"is_quick_play_singleplayer"`
	IsQuickPlayMultiplayer  bool `json:"is_quick_play_multiplayer"`
	IsQuickPlayRealms       bool `json:"is_quick_play_realms"`
}

type GameArgumentUnmarshal struct {
	GameArgument
}

func (g *GameArgumentUnmarshal) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch raw.(type) {
	case string:
		var str GameArgumentString
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		g.GameArgument = str
	case map[string]any:
		var rule GameArgumentRule
		if err := json.Unmarshal(data, &rule); err != nil {
			return err
		}
		g.GameArgument = rule
	default:
		return errors.New("invalid game argument type")
	}

	return nil
}

type JvmArgument interface {
	MarshalJSON() ([]byte, error)
	jvmArgument()
}

type JvmArgumentString string

func (j JvmArgumentString) String() string {
	return string(j)
}

func (j JvmArgumentString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(j))
}
func (j JvmArgumentString) jvmArgument() {}

type JvmArgumentRule struct {
	Rules []JvmArgumentRuleType `json:"rules"`
	Value ArgumentValue         `json:"value"`
}

func (j JvmArgumentRule) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Rules []JvmArgumentRuleType `json:"rules"`
		Value ArgumentValue         `json:"value"`
	}{
		Rules: j.Rules,
		Value: j.Value,
	})
}

func (j JvmArgumentRule) jvmArgument() {}

type JvmArgumentRuleType struct {
	Action   RuleAction            `json:"action"`
	Features any                   `json:"features"` // Unused
	OS       JvmArgumentRuleTypeOS `json:"os"`
}

type JvmArgumentRuleTypeOS struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

func (j JvmArgumentRuleTypeOS) Matched() bool {
	if j.Name == "" && j.Version == "" && j.Arch == "" {
		return true
	}
	if j.Name != "" && j.Name != "windows" && j.Name != "linux" && j.Name != "osx" {
		return false
	}
	if j.Version != "" && j.Version != osinfo.GetOsVersion() {
		return false
	}
	if j.Arch != "" && j.Arch != osArch() {
		return false
	}
	return true
}

type JvmArgumentUnmarshal struct {
	JvmArgument
}

func (j *JvmArgumentUnmarshal) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch raw.(type) {
	case string:
		var str JvmArgumentString
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		j.JvmArgument = str
	case map[string]any:
		var rule JvmArgumentRule
		if err := json.Unmarshal(data, &rule); err != nil {
			return err
		}
		j.JvmArgument = rule
	default:
		return errors.New("invalid jvm argument type")
	}

	return nil
}

type AssetIndex struct {
	ID        string `json:"id"`
	Sha1      string `json:"sha1"`
	Size      int    `json:"size"`
	TotalSize int    `json:"totalSize"`
	URL       string `json:"url"`
}

type Downloads struct {
	Client         Download `json:"client"`
	ClientMappings Download `json:"client_mappings"`
	Server         Download `json:"server"`
	ServerMappings Download `json:"server_mappings"`
	WindowsServer  Download `json:"windows_server,omitempty"`
}

type Download struct {
	Sha1 string `json:"sha1"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}

type JavaVersion struct {
	Component    string `json:"component"`
	MajorVersion int    `json:"majorVersion"`
}

type Library struct {
	Downloads LibraryDownloads  `json:"downloads"`
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Natives   map[string]string `json:"natives,omitempty"`
	Extract   LibraryExtract    `json:"extract,omitempty"`
	Rules     []LibraryRule     `json:"rules,omitempty"`
}

type LibraryArtifact struct {
	Path string `json:"path"`
	Sha1 string `json:"sha1"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}

type LibraryDownloads struct {
	Artifact    LibraryArtifact            `json:"artifact"`
	Classifiers map[string]LibraryArtifact `json:"classifiers"`
}

type LibraryExtract struct {
	Exclude []string `json:"exclude"`
}

type LibraryRule struct {
	Action RuleAction            `json:"action"`
	Os     JvmArgumentRuleTypeOS `json:"os"`
}

type Logging struct {
	Client LoggingClient `json:"client"`
}

type LoggingClient struct {
	Argument string            `json:"argument"`
	File     LoggingClientFile `json:"file"`
	Type     LoggingClientType `json:"type"`
}

type LoggingClientType string

const (
	LoggingClientTypeLog4j2Xml LoggingClientType = "log4j2-xml"
)

type LoggingClientFile struct {
	ID   string `json:"id"`
	Sha1 string `json:"sha1"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}
