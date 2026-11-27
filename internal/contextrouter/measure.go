package contextrouter

import (
	"context"
	"os"
	"strings"
)

// ReferenceCommit is the agents-context-router revision the size rule follows.
const ReferenceCommit = "19c94f9"

// Limits bound the work one report may do. The config is untrusted: one topic
// can name the same large file many times.
type Limits struct {
	MaxConfigBytes int64
	MaxFileBytes   int64
	// MaxScanBytes caps the file bytes scanned for bodies in one report.
	MaxScanBytes int64
	MaxTopics    int
	// MaxSources caps source entries across all topics.
	MaxSources int
}

// DefaultLimits is what the dashboard uses.
var DefaultLimits = Limits{MaxConfigBytes: 1 << 20, MaxFileBytes: 1 << 20, MaxScanBytes: 64 << 20, MaxTopics: 500, MaxSources: 2000}

// Why the config could not be used.
const (
	ConfigUnreadable = "config_unreadable" // Detail is a file error code
	ConfigInvalid    = "config_invalid"    // Detail is the parse message
)

// Why a topic has no size.
const (
	ReasonSource = "source" // one of its sources failed
	ReasonLimit  = "limit"  // the report ran out of its Limits
	// ReasonCompacted: the config compacts path links, which is not measured.
	ReasonCompacted = "compacted"
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
	Root      string   `json:"root"`
	Config    string   `json:"config"`
	Reference string   `json:"reference"`
	Error     *Problem `json:"error,omitempty"`
	Kernel    *Kernel  `json:"kernel,omitempty"`
	Topics    []Topic  `json:"topics"`
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
	root    *os.Root
	lim     Limits
	cfg     routerCfg
	files   map[string]fileResult
	bodies  map[bodyKey]bodyResult
	scanned int64
}

func (m *measurer) file(rel string) fileResult {
	if cached, ok := m.files[rel]; ok {
		return cached
	}
	text, code := readRegular(m.root, rel, m.lim.MaxFileBytes)
	result := fileResult{text, code}
	m.files[rel] = result
	return result
}

// body returns one source's body length. ok is false when the scan budget is spent.
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
	file := m.file(rel)
	if file.code != "" {
		result = bodyResult{code: file.code}
	} else {
		if m.scanned+int64(len(file.text)) > m.lim.MaxScanBytes {
			return bodyResult{}, false
		}
		m.scanned += int64(len(file.text))
		text, hits := section(file.text, sectionOpts{Heading: src.Heading, Strip: strip, Frontmatter: src.Frontmatter})
		if hits != 1 {
			result = bodyResult{code: ErrHeading, hits: hits}
		} else {
			result = bodyResult{bytes: len(text)}
		}
	}
	m.bodies[key] = result
	return result, true
}

// Measure reads the router in root. The error is non-nil only when ctx ended
// or root cannot be opened; everything else is in the report.
func Measure(ctx context.Context, root string, lim Limits) (Report, error) {
	report := Report{Root: root, Config: ConfigRel, Reference: ReferenceCommit, Topics: []Topic{}, Mounts: []Mount{}}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return report, err
	}
	defer dir.Close()

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
	m := &measurer{root: dir, lim: lim, cfg: cfg, files: map[string]fileResult{}, bodies: map[bodyKey]bodyResult{}}

	kernel := &Kernel{Path: cfg.RootInstructions, Cap: cfg.KernelCap}
	report.Kernel = kernel
	kernelText, kernelRead := "", false
	if rel, inside := cleanRel(cfg.RootInstructions); !inside {
		kernel.Error = ErrOutside
	} else if file := m.file(rel); file.code != "" {
		kernel.Error = file.code
	} else {
		size := len(file.text)
		kernel.Bytes, kernelText, kernelRead = &size, file.text, true
	}

	sources := 0
	for index, tc := range cfg.Topics {
		topic := Topic{Name: tc.Name, Description: tc.Description, Cap: cfg.TopicCap, Sources: []Source{}}
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
			if topic.Reason == ReasonLimit || index >= lim.MaxTopics || sources > lim.MaxSources {
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
		if topic.Reason == "" && cfg.Compact {
			topic.Reason = ReasonCompacted
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
