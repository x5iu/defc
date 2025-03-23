package defc

import (
	"html/template"
	"io"
	"strings"
	"testing"
)

// The bench results show that for ordinary template rendering tasks (such as a single CRUD SQL statement),
// using `bind` takes 10 times longer than using `arguments`, but the time spent is still in the microsecond
// range and can be ignored. However, as the content of the template rendering gradually increases, using
// `bind` will take more and more time and memory; at its peak (i.e., Benchmark_1000), using `bind` requires
// 700 milliseconds, while using `arguments` still maintains a rendering time of 1 microsecond.
//
// Use `arguments` to add parameters whenever possible instead of `bind`.
//
// ```
// goos: darwin
// goarch: arm64
// pkg: github.com/x5iu/defc/runtime
// cpu: Apple M1
// BenchmarkBind1-8                   64046             15810 ns/op           15786 B/op        215 allocs/op
// BenchmarkArgumentsBind1-8         918460              1224 ns/op             920 B/op         19 allocs/op
// BenchmarkBind10-8                   6634            163205 ns/op          154695 B/op       3175 allocs/op
// BenchmarkArgumentsBind10-8        904687              1258 ns/op             920 B/op         19 allocs/op
// BenchmarkBind100-8                   100          11570121 ns/op         9696021 B/op     176360 allocs/op
// BenchmarkArgumentsBind100-8       814221              1264 ns/op             921 B/op         19 allocs/op
// BenchmarkBind1000-8                    2         722617750 ns/op        969184652 B/op  16180071 allocs/op
// BenchmarkArgumentsBind1000-8      812119              1487 ns/op             927 B/op         19 allocs/op
// ```

type Person struct {
	Name    string
	Age     int
	Gender  string
	Address string
}

func BenchmarkBind1(b *testing.B)             { benchmarkBind(b, 1) }
func BenchmarkArgumentsBind1(b *testing.B)    { benchmarkArgumentsBind(b, 1) }
func BenchmarkBind10(b *testing.B)            { benchmarkBind(b, 10) }
func BenchmarkArgumentsBind10(b *testing.B)   { benchmarkArgumentsBind(b, 10) }
func BenchmarkBind100(b *testing.B)           { benchmarkBind(b, 100) }
func BenchmarkArgumentsBind100(b *testing.B)  { benchmarkArgumentsBind(b, 100) }
func BenchmarkBind1000(b *testing.B)          { benchmarkBind(b, 1000) }
func BenchmarkArgumentsBind1000(b *testing.B) { benchmarkArgumentsBind(b, 1000) }

func benchmarkBind(b *testing.B, n int) {
	b.ReportAllocs()
	const tmplStr = `
	{{ bind .Name }}
	{{ bind .Age }}
	{{ bind .Gender }}
	{{ bind .Address }}
		`
	var largeTmplStr strings.Builder
	for i := 0; i < n; i++ {
		largeTmplStr.WriteString(tmplStr)
	}
	for i := 0; i < b.N; i++ {
		var argListBenchmarkBind []any
		bind := func(arg any) string {
			argListBenchmarkBind = append(argListBenchmarkBind, arg)
			return BindVars(len(MergeArgs(argListBenchmarkBind)))
		}
		funcMap := template.FuncMap{
			"bind": bind,
		}
		t := template.Must(template.New("BenchmarkBind").Funcs(funcMap).Parse(largeTmplStr.String()))
		t.Execute(io.Discard, Person{
			Name:    "John",
			Age:     20,
			Gender:  "Male",
			Address: "123 Main St, Anytown, USA",
		})
	}
}

func benchmarkArgumentsBind(b *testing.B, n int) {
	b.ReportAllocs()
	const tmplStr = `
	{{ .args.Bind .Name }}
	{{ .args.Bind .Age }}
	{{ .args.Bind .Gender }}
	{{ .args.Bind .Address }}
	`
	var largeTmplStr strings.Builder
	for i := 0; i < n; i++ {
		largeTmplStr.WriteString(tmplStr)
	}
	t := template.Must(template.New("BenchmarkArgumentsBind").Parse(largeTmplStr.String()))
	for i := 0; i < b.N; i++ {
		var argListBenchmarkArgumentsBind Arguments
		t.Execute(io.Discard, map[string]any{
			"args":    argListBenchmarkArgumentsBind,
			"Name":    "John",
			"Age":     20,
			"Gender":  "Male",
			"Address": "123 Main St, Anytown, USA",
		})
	}
}
