// Copyright 2021 syzkaller project authors. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/syzkaller/prog"
)

func TestVerify(t *testing.T) {
	p := "breaks_returns()\n" +
		"minimize$0(0x1, 0x1)\n" +
		"test$res0()\n"
	tests := []struct {
		name       string
		res        []*Result
		wantReport *ResultReport
		wantStats  *Stats
	}{
		{
			name: "mismatches not found in results",
			res: []*Result{
				makeResult(2, []int{11, 33, 22}, []int{1, 3, 3}...),
				makeResult(4, []int{11, 33, 22}, []int{1, 3, 3}...)},
			wantReport: nil,
			wantStats: &Stats{
				Calls: map[string]*CallStats{
					"breaks_returns": {Name: "breaks_returns", Occurrences: 1, States: map[ReturnState]bool{}},
					"minimize$0":     {Name: "minimize$0", Occurrences: 1, States: map[ReturnState]bool{}},
					"test$res0":      {Name: "test$res0", Occurrences: 1, States: map[ReturnState]bool{}},
				},
			},
		},
		{
			name: "mismatches found in results",
			res: []*Result{
				makeResult(1, []int{1, 3, 2}, []int{4, 7, 7}...),
				makeResult(4, []int{1, 3, 5}, []int{4, 7, 3}...),
			},
			wantReport: &ResultReport{
				Prog: p,
				Reports: []*CallReport{
					{Call: "breaks_returns", States: map[int]ReturnState{1: {1, 4}, 4: {1, 4}}},
					{Call: "minimize$0", States: map[int]ReturnState{1: {3, 7}, 4: {3, 7}}},
					{Call: "test$res0", States: map[int]ReturnState{1: {2, 7}, 4: {5, 3}}, Mismatch: true},
				},
			},
			wantStats: &Stats{
				TotalMismatches: 1,
				Calls: map[string]*CallStats{
					"breaks_returns": {Name: "breaks_returns", Occurrences: 1, States: map[ReturnState]bool{}},
					"minimize$0":     {Name: "minimize$0", Occurrences: 1, States: map[ReturnState]bool{}},
					"test$res0": {Name: "test$res0", Occurrences: 1,
						Mismatches: 1, States: map[ReturnState]bool{
							{Errno: 2, Flags: 7}: true,
							{Errno: 5, Flags: 3}: true}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := prog.InitTargetTest(t, "test", "64")
			prog, err := target.Deserialize([]byte(p), prog.Strict)
			if err != nil {
				t.Fatalf("failed to deserialise test program: %v", err)
			}
			stats := emptyTestStats()
			got := Verify(test.res, prog, stats)
			if diff := cmp.Diff(test.wantReport, got); diff != "" {
				t.Errorf("Verify report mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.wantStats, stats); diff != "" {
				t.Errorf("Verify stats mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
