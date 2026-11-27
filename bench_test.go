// Copyright (c) 2026 the go-widgets/svg authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svg

import "testing"

func benchAssemble(b *testing.B, n int, mode string) {
	fs := scenario(b, n, mode)
	total := 0
	for _, f := range fs {
		total += len(f.SVG)
	}
	b.SetBytes(int64(total))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Animate(fs, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAssemble60LiveRegion(b *testing.B) { benchAssemble(b, 60, "live") }
func BenchmarkAssemble60Static(b *testing.B)     { benchAssemble(b, 60, "static") }
func BenchmarkAssemble60AllChange(b *testing.B)  { benchAssemble(b, 60, "all") }
