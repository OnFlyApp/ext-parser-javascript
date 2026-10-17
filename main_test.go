package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func kinds(t *testing.T, file, src string) map[string]Chunk {
	t.Helper()
	chunks, err := parse(file, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Chunk{}
	for _, c := range chunks {
		key := c.Type + " " + c.Metadata["fqn"]
		if c.Metadata["fqn"] == "" {
			key = c.Type + " " + c.Metadata["name"]
		}
		out[key] = c
	}
	return out
}

func keys(m map[string]Chunk) []string {
	var k []string
	for s := range m {
		k = append(k, s)
	}
	return k
}

func TestAllChunkTypes(t *testing.T) {
	src, err := os.ReadFile("testdata/all.js")
	if err != nil {
		t.Fatal(err)
	}
	got := kinds(t, "src/all.js", string(src))
	for _, want := range []string{
		`import ./dep`, `import react`, `export ./z`, `class Foo`, `property Foo.x`, `property Foo.#z`,
		`method Foo.constructor`, `method Foo.v`, `method Foo.handle`, `method Foo.it`, `static_block Foo.static`,
		`function f`, `function g`, `function h`, `variable x`, `constant K`, `function Comp`, `statement statements`,
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing chunk %q; have %v", want, keys(got))
		}
	}
	if c := got["class Foo"]; strings.Contains(c.Content, "constructor") || c.Metadata["extends"] != "Bar" ||
		c.Metadata["decorators"] != `@component({ selector: "x" })` || c.Metadata["exported"] != "true" {
		t.Errorf("class header chunk wrong: %+v", c)
	}
	if c := got["method Foo.v"]; c.Metadata["receiver"] != "Foo" || c.Metadata["modifiers"] != "async get" || !strings.HasPrefix(c.Content, "/** Value. */") {
		t.Errorf("method chunk wrong: %+v", c)
	}
	if c := got["method Foo.it"]; c.Metadata["name"] != "it" || !strings.Contains(c.Metadata["modifiers"], "async") {
		t.Errorf("async generator method wrong: %+v", c)
	}
	if c := got["function f"]; c.Metadata["modifiers"] != "async" || c.Metadata["params"] != "(a)" {
		t.Errorf("arrow function chunk wrong: %+v", c)
	}
	if c := got["property Foo.#z"]; c.Metadata["visibility"] != "private" {
		t.Errorf("private field wrong: %+v", c)
	}
}

func TestDefaultExports(t *testing.T) {
	got := kinds(t, "a.js", "export default class {}\n")
	if c, ok := got["class default"]; !ok || c.Metadata["default"] != "true" {
		t.Errorf("default class: %v", keys(got))
	}
	got = kinds(t, "a.js", "export default function () {}\n")
	if c, ok := got["function default"]; !ok || c.Metadata["default"] != "true" {
		t.Errorf("default function: %v", keys(got))
	}
	got = kinds(t, "a.js", "const Foo = 1;\nexport default Foo;\n")
	if _, ok := got["export default"]; !ok {
		t.Errorf("default expression: %v", keys(got))
	}
}

func TestOddInputs(t *testing.T) {
	for name, src := range map[string]string{
		"shebang": "#!/usr/bin/env node\nexport function a() {}\n",
		"bom":     "\ufeffexport function a() {}\n",
		"crlf":    "export function a() {\r\n  return 1;\r\n}\r\n",
	} {
		if got := kinds(t, "a.js", src); got["function a"].Type == "" {
			t.Errorf("%s: lost function a; have %v", name, keys(got))
		}
	}
	got := kinds(t, "a.js", "class C { [Symbol.iterator]() {} }\n")
	if _, ok := got["method C.[Symbol.iterator]"]; !ok {
		t.Errorf("computed method name lost; have %v", keys(got))
	}
}

func TestMultipleDecorators(t *testing.T) {
	got := kinds(t, "a.js", "@A({ x: 1 })\n@B()\nexport class C { @Input() @HostBinding('x') p = 1; }\n")
	if d := got["class C"].Metadata["decorators"]; d != "@A({ x: 1 }) @B()" {
		t.Errorf("class decorators = %q", d)
	}
	if d := got["property C.p"].Metadata["decorators"]; d != "@Input() @HostBinding('x')" {
		t.Errorf("property decorators = %q", d)
	}
}

func TestJSX(t *testing.T) {
	got := kinds(t, "src/c.jsx", "export const A = () => <div>hi</div>;\nexport function B() { return <b/>; }\n")
	for _, want := range []string{"function A", "function B"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing %q; have %v", want, keys(got))
		}
	}
}

func TestEmptyAndCommentOnly(t *testing.T) {
	if chunks, _ := parse("a.js", nil); chunks == nil || len(chunks) != 0 {
		t.Fatalf("empty file must give [], got %v", chunks)
	}
	chunks, err := parse("a.js", []byte("// just a comment\n"))
	if err != nil || len(chunks) != 1 || chunks[0].Type != "file" {
		t.Fatalf("comment-only file must be one file chunk, got %v, err %v", chunks, err)
	}
	b, _ := json.Marshal(chunks)
	if !strings.HasPrefix(string(b), "[") {
		t.Fatal("not an array")
	}
}

func TestSyntaxErrorKeepsDeclarations(t *testing.T) {
	got := kinds(t, "a.js", "function ok() {}\nclass { broken\nfunction after() {}\n")
	if _, ok := got["function ok"]; !ok {
		t.Errorf("lost declaration before the error; have %v", keys(got))
	}
}

func TestLongBodySplits(t *testing.T) {
	body := strings.Repeat("  console.log('line');\n", 600)
	chunks, err := parse("a.js", []byte("function big() {\n"+body+"}\n"))
	if err != nil || len(chunks) < 2 {
		t.Fatalf("expected split parts, got %d chunks, err %v", len(chunks), err)
	}
	for _, c := range chunks {
		if c.Metadata["part"] == "" || c.Metadata["name"] != "big" {
			t.Fatalf("bad part chunk: %+v", c.Metadata)
		}
	}
}

func TestSplitPartsHaveOwnLines(t *testing.T) {
	chunks, _ := parse("a.js", []byte("function big() {\n"+strings.Repeat("  console.log('line');\n", 600)+"}\n"))
	if len(chunks) < 2 || chunks[0].Metadata["end_line"] == chunks[1].Metadata["end_line"] ||
		chunks[1].Metadata["start_line"] != strconv.Itoa(atoi(chunks[0].Metadata["end_line"])+1) {
		t.Fatalf("part lines wrong: %v %v", chunks[0].Metadata, chunks[1].Metadata)
	}
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func TestLineCommentKept(t *testing.T) {
	got := kinds(t, "a.js", "// note\nfunction g() {}\n\n// far\n\nfunction h() {}\n")
	if !strings.HasPrefix(got["function g"].Content, "// note") || strings.HasPrefix(got["function h"].Content, "//") {
		t.Errorf("comment handling wrong: %q / %q", got["function g"].Content, got["function h"].Content)
	}
}

func TestOversizedFileBecomesPlainText(t *testing.T) {
	src := strings.Repeat("// padding padding padding\n", maxParseSize/20)
	chunks, err := parse("big.js", []byte(src))
	if err != nil || len(chunks) < 2 {
		t.Fatalf("want split file chunks, got %d, err %v", len(chunks), err)
	}
	for _, c := range chunks {
		if c.Type != "file" || c.Metadata["reason"] != "too_large" {
			t.Fatalf("unexpected chunk %s %v", c.Type, c.Metadata)
		}
	}
}

func TestHugeFileIsSkipped(t *testing.T) {
	chunks, err := parse("huge.js", []byte(strings.Repeat("x", maxFallbackSize+1)))
	if err != nil || chunks == nil || len(chunks) != 0 {
		t.Fatalf("want [], got %d chunks, err %v", len(chunks), err)
	}
}

func TestSplitKeepsValidUTF8(t *testing.T) {
	body := strings.Repeat("const s = 'ёёёёёёёёёёёёёёёёёёёё'; ", 600) // one long line: forces the no-newline fallback
	chunks, err := parse("a.js", []byte("function u() { "+body+"}\n"))
	if err != nil || len(chunks) < 2 {
		t.Fatalf("expected split, got %d, err %v", len(chunks), err)
	}
	for _, c := range chunks {
		if !utf8.ValidString(c.Content) {
			t.Fatalf("invalid utf-8 in part %s", c.Metadata["part"])
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte("export class A { m() {} }"))
	f.Add([]byte("const a = <div/>;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		// Syntax garbage is fine; a recovered panic is not.
		if _, err := parse("f.js", b); err != nil && strings.Contains(err.Error(), "panic") {
			t.Fatal(err)
		}
	})
}

// Golden files pin content and metadata, not just chunk presence. Regenerate
// with: go test -tags "grammar_subset grammar_subset_javascript" -run Golden -update
var update = flag.Bool("update", false, "rewrite golden files")

func TestGolden(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.js")
	jsx, _ := filepath.Glob("testdata/*.jsx")
	files = append(files, jsx...)
	if len(files) == 0 {
		t.Fatal("no fixtures")
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		chunks, err := parse("src/"+filepath.Base(f), src)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := json.MarshalIndent(chunks, "", "  ")
		golden := strings.TrimSuffix(f, filepath.Ext(f)) + ".golden.json"
		if *update {
			if err := os.WriteFile(golden, append(got, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run with -update)", err)
		}
		if strings.TrimSpace(string(want)) != string(got) {
			t.Errorf("%s differs from %s (run with -update if intended)", f, golden)
		}
	}
}
