package mcp

import (
	"encoding/json"
	"math/big"
	"sort"
	"strings"
)

// compareContracts reports every changed dimension. Its grades describe a
// small supported schema fragment, not schema implication or tool safety.
func compareContracts(old, now map[string]ToolContract) ([]ContractChange, int) {
	changes := []ContractChange{}
	omitted := 0
	add := func(tool, area, path, grade, message string) {
		if len(changes) >= 200 {
			omitted++
			return
		}
		path, message = contractLocation(path, message)
		changes = append(changes, ContractChange{Tool: tool, Area: area, Path: path, Grade: grade, Message: message})
	}
	names := map[string]bool{}
	for k := range old {
		names[k] = true
	}
	for k := range now {
		names[k] = true
	}
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		before, had := old[name]
		after, has := now[name]
		if !had {
			add(name, "tool", "", "EXPANDED", "tool added; review required")
			continue
		}
		if !has {
			add(name, "tool", "", "BREAKING", "tool removed; review required")
			continue
		}
		if before.Hash == after.Hash {
			continue
		}
		a, ae := contractJSON(before.Definition)
		b, be := contractJSON(after.Definition)
		am, aok := a.(map[string]any)
		bm, bok := b.(map[string]any)
		if ae != nil || be != nil || !aok || !bok {
			add(name, "contract", "", "UNKNOWN", "contract changed; comparison unavailable")
			continue
		}
		count := len(changes) + omitted
		if before.InputHash != after.InputHash {
			contractSchemaDiff(am["inputSchema"], bm["inputSchema"], "/inputSchema", false, func(path, grade, msg string) { add(name, "input", path, grade, msg) })
		}
		if before.OutputHash != after.OutputHash {
			contractJSONDiff(am["outputSchema"], bm["outputSchema"], "/outputSchema", func(path string) { add(name, "output", path, "UNKNOWN", "output contract changed; review required") })
		}
		for _, key := range []string{"description", "title"} {
			if !contractEqualField(am, bm, key) {
				add(name, "prose", "/"+key, "DESCRIPTION", "model-visible text changed; review required")
			}
		}
		metaKeys := map[string]bool{}
		for k := range am {
			metaKeys[k] = true
		}
		for k := range bm {
			metaKeys[k] = true
		}
		ordered := make([]string, 0, len(metaKeys))
		for k := range metaKeys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			switch key {
			case "inputSchema", "outputSchema", "description", "title":
				continue
			}
			if !contractEqualField(am, bm, key) {
				_, ah := am[key]
				_, bh := bm[key]
				if ah != bh {
					add(name, "metadata", contractPointer("", key), "METADATA", "metadata changed; review required")
					continue
				}
				contractJSONDiff(am[key], bm[key], contractPointer("", key), func(path string) { add(name, "metadata", path, "METADATA", "metadata changed; review required") })
			}
		}
		if len(changes)+omitted == count {
			add(name, "contract", "", "UNKNOWN", "contract fingerprint changed; review required")
		}
	}
	return changes, omitted
}
func contractEqual(a, b any) bool { return contractHash(a) == contractHash(b) }
func contractEqualField(a, b map[string]any, k string) bool {
	av, ak := a[k]
	bv, bk := b[k]
	return ak == bk && contractEqual(av, bv)
}
func contractUnionKeys(a, b map[string]any) []string {
	m := map[string]any{}
	for k := range a {
		m[k] = nil
	}
	for k := range b {
		m[k] = nil
	}
	return contractKeys(m)
}

func contractJSONDiff(a, b any, path string, emit func(string)) {
	if contractEqual(a, b) {
		return
	}
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		for _, k := range contractUnionKeys(am, bm) {
			if !contractEqualField(am, bm, k) {
				_, ah := am[k]
				_, bh := bm[k]
				if ah != bh {
					emit(contractPointer(path, k))
					continue
				}
				contractJSONDiff(am[k], bm[k], contractPointer(path, k), emit)
			}
		}
		return
	}
	emit(path)
}

// Unsupported keywords invalidate directional proofs even when unchanged:
// references, not/applicators and unevaluated constraints alter local meaning.
func contractSimpleSchema(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for k, v := range m {
		switch k {
		case "type", "required", "enum", "const", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties", "title", "description", "examples", "default", "$comment":
		case "additionalProperties":
			if _, ok := v.(bool); !ok {
				return false
			}
		case "properties":
			children, ok := v.(map[string]any)
			if !ok {
				return false
			}
			for _, child := range children {
				if !contractSimpleSchema(child) {
					return false
				}
			}
		case "items":
			if !contractSimpleSchema(v) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
func contractSchemaText(k string) bool {
	return k == "description" || k == "title" || k == "examples" || k == "$comment"
}

func contractSchemaDiff(a, b any, path string, uncertain bool, emit func(string, string, string)) {
	if contractEqual(a, b) {
		return
	}
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if !aok || !bok {
		emit(path, "UNKNOWN", "schema structure changed; review required")
		return
	}
	uncertain = uncertain || !contractSimpleSchema(am) || !contractSimpleSchema(bm)
	changed := 0
	for _, k := range contractUnionKeys(am, bm) {
		if !contractEqualField(am, bm, k) && !contractSchemaText(k) {
			changed++
		}
	}
	uncertain = uncertain || changed > 1
	for _, k := range contractUnionKeys(am, bm) {
		if contractEqualField(am, bm, k) {
			continue
		}
		av, had := am[k]
		bv, has := bm[k]
		p := contractPointer(path, k)
		if contractSchemaText(k) {
			emit(p, "NOTATION", "schema text changed; review required")
			continue
		}
		if k == "properties" {
			ap, aok := av.(map[string]any)
			bp, bok := bv.(map[string]any)
			if !had {
				ap = map[string]any{}
				aok = true
			}
			if !has {
				bp = map[string]any{}
				bok = true
			}
			if aok && bok {
				for _, prop := range contractUnionKeys(ap, bp) {
					x, xok := ap[prop]
					y, yok := bp[prop]
					if xok == yok && contractEqual(x, y) {
						continue
					}
					pp := contractPointer(p, prop)
					if xok && yok {
						contractSchemaDiff(x, y, pp, uncertain, emit)
						continue
					}
					grade := "UNKNOWN"
					// Closed simple objects with unchanged required sets admit a proof.
					if !uncertain && am["additionalProperties"] == false && bm["additionalProperties"] == false {
						if !xok {
							grade = "EXPANDED"
						} else {
							grade = "BREAKING"
						}
					}
					emit(pp, grade, "object property changed; review required")
				}
				continue
			}
		}
		if k == "items" && had && has {
			contractSchemaDiff(av, bv, p, uncertain, emit)
			continue
		}
		grade := "UNKNOWN"
		if !uncertain {
			grade = contractKeywordGrade(k, av, bv, had, has)
		}
		emit(p, grade, "schema constraint changed; review required")
	}
}

func contractKeywordGrade(k string, a, b any, had, has bool) string {
	switch k {
	case "required":
		if !had {
			a = []any{}
		}
		if !has {
			b = []any{}
		}
		return contractSetGrade(a, b, "BREAKING", "LOOSENED", false)
	case "type":
		if had && has {
			return contractSetGrade(contractTypes(a), contractTypes(b), "EXPANDED", "BREAKING", true)
		}
	case "enum":
		if had && has {
			// Fingerprints deliberately preserve numeric lexemes. JSON Schema
			// numeric equality does not: 1 and 1.0 are one enum value, even in
			// literal objects/arrays. Do not use lexical subset proofs there.
			if contractContainsNumber(a) || contractContainsNumber(b) {
				return "UNKNOWN"
			}
			return contractSetGrade(a, b, "EXPANDED", "BREAKING", false)
		}
	case "additionalProperties":
		if !had {
			a = true
		}
		if !has {
			b = true
		}
		if a == false && b == true {
			return "LOOSENED"
		}
		if a == true && b == false {
			return "BREAKING"
		}
	case "minimum", "exclusiveMinimum", "minLength", "minItems", "minProperties", "maximum", "exclusiveMaximum", "maxLength", "maxItems", "maxProperties":
		if !had {
			return "BREAKING"
		}
		if !has {
			return "LOOSENED"
		}
		x, xok := a.(json.Number)
		y, yok := b.(json.Number)
		if !xok || !yok {
			return "UNKNOWN"
		}
		cmp := contractCompareNumber(x, y)
		if cmp == 0 {
			return "UNKNOWN"
		}
		lower := strings.HasPrefix(k, "min") || k == "exclusiveMinimum"
		if (lower && cmp < 0) || (!lower && cmp > 0) {
			return "BREAKING"
		}
		return "LOOSENED"
	}
	return "UNKNOWN"
}

func contractContainsNumber(v any) bool {
	switch v := v.(type) {
	case json.Number:
		return true
	case map[string]any:
		for _, child := range v {
			if contractContainsNumber(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if contractContainsNumber(child) {
				return true
			}
		}
	}
	return false
}

func contractTypes(v any) any {
	if s, ok := v.(string); ok {
		return []any{s}
	}
	return v
}
func contractSetGrade(a, b any, added, removed string, types bool) string {
	aa, aok := a.([]any)
	bb, bok := b.([]any)
	if !aok || !bok {
		return "UNKNOWN"
	}
	subset := func(x, y []any) bool {
		// Inventory bounds alone do not make quadratic enum comparisons safe.
		members := make(map[string]bool, len(y))
		for _, v := range y {
			members[contractHash(v)] = true
		}
		number := members[contractHash("number")]
		for _, v := range x {
			if members[contractHash(v)] {
				continue
			}
			if s, ok := v.(string); types && ok && s == "integer" && number {
				continue
			}
			return false
		}
		return true
	}
	ab, ba := subset(aa, bb), subset(bb, aa)
	if ab && !ba {
		return added
	}
	if ba && !ab {
		return removed
	}
	return "UNKNOWN"
}

// Decimal representation avoids float rounding and exponent-sized allocation.
// Exponents themselves use big.Int so legal huge exponent lexemes stay bounded
// by wire length, rather than requesting 10^exponent bytes of memory.
type contractDecimal struct {
	negative bool
	digits   string
	exponent *big.Int
}

func contractNumber(n json.Number) contractDecimal {
	s := string(n)
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	e := new(big.Int)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e.SetString(s[i+1:], 10)
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		e.Sub(e, big.NewInt(int64(len(s)-i-1)))
		s = s[:i] + s[i+1:]
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return contractDecimal{digits: "0", exponent: new(big.Int)}
	}
	trimmed := strings.TrimRight(s, "0")
	e.Add(e, big.NewInt(int64(len(s)-len(trimmed))))
	return contractDecimal{negative: negative, digits: trimmed, exponent: e}
}
func contractNonnegativeInteger(n json.Number) bool {
	d := contractNumber(n)
	return !d.negative && d.exponent.Sign() >= 0
}
func contractCompareNumber(a, b json.Number) int {
	x, y := contractNumber(a), contractNumber(b)
	if x.digits == "0" && y.digits == "0" {
		return 0
	}
	if x.negative != y.negative {
		if x.negative {
			return -1
		}
		return 1
	}
	cmp := 0
	if x.digits == "0" {
		cmp = -1
	} else if y.digits == "0" {
		cmp = 1
	} else {
		xm := new(big.Int).Add(x.exponent, big.NewInt(int64(len(x.digits))))
		ym := new(big.Int).Add(y.exponent, big.NewInt(int64(len(y.digits))))
		cmp = xm.Cmp(ym)
		if cmp == 0 {
			width := max(len(x.digits), len(y.digits))
			cmp = strings.Compare(x.digits+strings.Repeat("0", width-len(x.digits)), y.digits+strings.Repeat("0", width-len(y.digits)))
		}
	}
	if x.negative {
		return -cmp
	}
	return cmp
}
