package contextrouter

import (
	"context"
	"os"
	"strings"
	"time"
)

// Limits bound the work one report may do. The config is untrusted: one topic
// can name the same large file many times, and many names can lead to one file.
type Limits struct {
	MaxConfigBytes int64
	MaxFileBytes   int64
	// MaxScanBytes caps the file bytes read in one report, and separately the
	// bytes scanned for bodies.
	MaxScanBytes int64
	// MaxCompactBytes caps one body that compactPathLinks rewrites. The link
	// scanner is quadratic on input built to stress it, so a body past this
	// is left unmeasured rather than scanned.
	MaxCompactBytes int
	MaxTopics       int
	// MaxSources caps source entries across all topics.
	MaxSources int
	// MaxDuration caps the time spent measuring; zero means no cap.
	MaxDuration time.Duration
}

// DefaultLimits is what the dashboard uses.
var DefaultLimits = Limits{
	MaxConfigBytes:  1 << 20,
	MaxFileBytes:    1 << 20,
	MaxScanBytes:    64 << 20,
	MaxCompactBytes: 64 << 10,
	MaxTopics:       500,
	MaxSources:      2000,
	MaxDuration:     5 * time.Second,
}

// Why the config could not be used.
const (
	ConfigUnreadable = "config_unreadable" // Detail is a file error code
	ConfigInvalid    = "config_invalid"    // Detail is the parse message
)

// Why a topic has no size.
const (
	ReasonSource = "source" // one of its sources failed
	ReasonLimit  = "limit"  // the report ran out of its Limits
)

// ErrHeading is a source whose heading does not match exactly once.
const ErrHeading = "heading"

type Problem struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Kernel is the file the config names as root instructions.
type Kernel struct {
	Path  string `json:"path"`
	Bytes *int   `json:"bytes"`
	Cap   int    `json:"cap"`
	Error string `json:"error,omitempty"`
}

type Source struct {
	Path    string `json:"path"`
	Heading string `json:"heading,omitempty"`
	// Bytes is the body alone, without the marker line the router prints above it.
	Bytes *int   `json:"bytes"`
	Error string `json:"error,omitempty"`
	// Count is how many headings matched, with Error "heading".
	Count *int `json:"count,omitempty"`
}

type Topic struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Bytes is nil when the router would print no size either; Reason says why.
	Bytes  *int   `json:"bytes"`
	Cap    int    `json:"cap"`
	Reason string `json:"reason,omitempty"`
	// Listed is whether the kernel names the topic; nil when the kernel is unreadable.
	Listed  *bool    `json:"listed"`
	Sources []Source `json:"sources"`
}

// Report is one folder's router: its kernel and its own topics. Mounts are
// only listed.
type Report struct {
	Root   string `json:"root"`
	Config string `json:"config"`
	// ScriptVersion is the version of the folder's own scripts/ai-context.py,
	// empty when there is none to read. SemanticsVersion is the router version
	// whose size rule was applied, and VersionStatus says how the two relate.
	ScriptVersion    string   `json:"scriptVersion"`
	SemanticsVersion string   `json:"semanticsVersion"`
	VersionStatus    string   `json:"versionStatus"`
	Error            *Problem `json:"error,omitempty"`
	Kernel           *Kernel  `json:"kernel,omitempty"`
	Topics           []Topic  `json:"topics"`
	// Truncated is set when Limits left some topics unmeasured.
	Truncated bool    `json:"truncated"`
	Mounts    []Mount `json:"mounts"`
}

type bodyKey struct {
	path        string
	heading     string
	hasHeading  bool
	strip       bool
	frontmatter bool
}

type bodyResult struct {
	bytes int
	code  string
	hits  int
}

type fileResult struct {
	text string
	code string
}

type measurer struct {
	root     *os.Root
	lim      Limits
	cfg      routerCfg
	files    map[string]fileResult
	bodies   map[bodyKey]bodyResult
	read     int64 // file bytes read and kept
	scanned  int64 // file bytes scanned for bodies
	deadline time.Time
}

// spent reports whether the time budget is used up.
func (m *measurer) spent() bool {
	return !m.deadline.IsZero() && time.Now().After(m.deadline)
}

// file reads rel once. ok is false when the read budget is spent: many names
// (symlinks inside the folder) can lead to one large file, and each is kept.
func (m *measurer) file(rel string) (result fileResult, ok bool) {
	if cached, done := m.files[rel]; done {
		return cached, true
	}
	if m.read >= m.lim.MaxScanBytes {
		return fileResult{}, false
	}
	text, code := readRegular(m.root, rel, m.lim.MaxFileBytes)
	m.read += int64(len(text))
	result = fileResult{text, code}
	m.files[rel] = result
	return result, true
}

// body returns one source's body length. ok is false when a budget is spent.
func (m *measurer) body(src sourceCfg) (result bodyResult, ok bool) {
	rel, inside := cleanRel(src.Path)
	if !inside {
		return bodyResult{code: ErrOutside}, true
	}
	strip := m.cfg.Strip
	if src.Strip != nil {
		strip = *src.Strip
	}
	key := bodyKey{path: rel, strip: strip, frontmatter: src.Frontmatter}
	if src.Heading != nil {
		key.heading, key.hasHeading = *src.Heading, true
	}
	if cached, done := m.bodies[key]; done {
		return cached, true
	}
	file, ok := m.file(rel)
	if !ok {
		return bodyResult{}, false
	}
	if file.code != "" {
		result = bodyResult{code: file.code}
	} else {
		if m.scanned+int64(len(file.text)) > m.lim.MaxScanBytes {
			return bodyResult{}, false
		}
		m.scanned += int64(len(file.text))
		text, hits := section(file.text, sectionOpts{Heading: src.Heading, Strip: strip, Frontmatter: src.Frontmatter})
		switch {
		case hits != 1:
			result = bodyResult{code: ErrHeading, hits: hits}
		case m.cfg.Compact && len(text) > m.lim.MaxCompactBytes:
			return bodyResult{}, false
		case m.cfg.Compact:
			result = bodyResult{bytes: len(compactPathLinks(text, rel))}
		default:
			result = bodyResult{bytes: len(text)}
		}
	}
	m.bodies[key] = result
	return result, true
}

// fallbackDescription mirrors topic_description: a topic without a description
// of its own shows the one in its first source's frontmatter, or nothing.
func (m *measurer) fallbackDescription(tc topicCfg) string {
	if len(tc.Sources) == 0 {
		return ""
	}
	rel, inside := cleanRel(tc.Sources[0].Path)
	if !inside {
		return ""
	}
	file, ok := m.file(rel)
	if !ok || file.code != "" {
		return ""
	}
	return pageDescription(strings.Split(file.text, "\n"))
}

// Measure reads the router in root. The error is non-nil only when ctx ended;
// everything else is in the report, a folder that is gone included.
func Measure(ctx context.Context, root string, lim Limits) (Report, error) {
	report := Report{Root: root, Config: ConfigRel, SemanticsVersion: SemanticsVersion, VersionStatus: VersionUnknown, Topics: []Topic{}, Mounts: []Mount{}}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		report.Error = &Problem{Code: ConfigUnreadable, Detail: ErrMissing}
		return report, nil
	}
	defer dir.Close()

	script, hasScript := scriptVersion(dir, lim.MaxFileBytes)
	if hasScript {
		report.ScriptVersion = script.String()
		switch {
		case script.less(stripAndCompactSince):
			// The folder's own check prints what its script computes, so follow that script.
			report.VersionStatus, report.SemanticsVersion = VersionLegacy, script.String()
		case script.less(firstUnseen):
			report.VersionStatus = VersionOK
		default:
			report.VersionStatus = VersionNewer
		}
	}

	text, code := readRegular(dir, ConfigRel, lim.MaxConfigBytes)
	if code != "" {
		report.Error = &Problem{Code: ConfigUnreadable, Detail: code}
		return report, nil
	}
	cfg, err := parseConfig(text)
	if err != nil {
		report.Error = &Problem{Code: ConfigInvalid, Detail: err.Error()}
		return report, nil
	}
	if cfg.Mounts != nil {
		report.Mounts = cfg.Mounts
	}
	if report.VersionStatus == VersionLegacy {
		// An older script does not know these options, so its sizes do not reflect them.
		cfg.Strip, cfg.Compact = false, false
		for t := range cfg.Topics {
			for i := range cfg.Topics[t].Sources {
				cfg.Topics[t].Sources[i].Strip = nil
				if script.less(sourceFrontmatterSince) {
					cfg.Topics[t].Sources[i].Frontmatter = false
				}
			}
		}
	}
	m := &measurer{root: dir, lim: lim, cfg: cfg, files: map[string]fileResult{}, bodies: map[bodyKey]bodyResult{}}
	if lim.MaxDuration > 0 {
		m.deadline = time.Now().Add(lim.MaxDuration)
	}

	kernel := &Kernel{Path: cfg.RootInstructions, Cap: cfg.KernelCap}
	report.Kernel = kernel
	kernelText, kernelRead := "", false
	if rel, inside := cleanRel(cfg.RootInstructions); !inside {
		kernel.Error = ErrOutside
	} else if file, _ := m.file(rel); file.code != "" {
		kernel.Error = file.code
	} else {
		size := len(file.text)
		kernel.Bytes, kernelText, kernelRead = &size, file.text, true
	}

	sources := 0
	for index, tc := range cfg.Topics {
		topic := Topic{Name: tc.Name, Description: tc.Description, Cap: cfg.TopicCap, Sources: []Source{}}
		if !tc.HasDescription {
			topic.Description = m.fallbackDescription(tc)
		}
		if tc.MaxBytes != nil {
			topic.Cap = *tc.MaxBytes
		}
		if kernelRead {
			listed := strings.Contains(kernelText, "`"+tc.Name+"`")
			topic.Listed = &listed
		}
		total := 0
		for i, sc := range tc.Sources {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			source := Source{Path: sc.Path}
			if sc.Heading != nil {
				source.Heading = *sc.Heading
			}
			sources++
			if topic.Reason == ReasonLimit || index >= lim.MaxTopics || sources > lim.MaxSources || m.spent() {
				topic.Reason = ReasonLimit
				topic.Sources = append(topic.Sources, source)
				continue
			}
			result, ok := m.body(sc)
			switch {
			case !ok:
				topic.Reason = ReasonLimit
			case result.code != "":
				source.Error = result.code
				if result.code == ErrHeading {
					hits := result.hits
					source.Count = &hits
				}
				topic.Reason = ReasonSource
			default:
				size := result.bytes
				source.Bytes = &size
				// "<!-- " path [" § " heading] " -->\n" body, joined by a blank line.
				total += len("<!-- ") + len(sc.Path) + len(" -->\n") + size
				if sc.Heading != nil {
					total += len(" § ") + len(*sc.Heading)
				}
				if i > 0 {
					total += 2
				}
			}
			topic.Sources = append(topic.Sources, source)
		}
		if index >= lim.MaxTopics {
			topic.Reason = ReasonLimit
		}
		if topic.Reason == "" {
			topic.Bytes = &total
		}
		if topic.Reason == ReasonLimit {
			report.Truncated = true
		}
		report.Topics = append(report.Topics, topic)
	}
	return report, nil
}
