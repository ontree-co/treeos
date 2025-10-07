package update

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
		desc     string
	}{
		// Beta version comparisons - the critical bug we're fixing
		{
			name:     "beta.9 vs beta.10",
			v1:       "0.1.0-beta.9",
			v2:       "0.1.0-beta.10",
			expected: -1,
			desc:     "beta.9 should be less than beta.10",
		},
		{
			name:     "beta.10 vs beta.9",
			v1:       "0.1.0-beta.10",
			v2:       "0.1.0-beta.9",
			expected: 1,
			desc:     "beta.10 should be greater than beta.9",
		},
		{
			name:     "beta.2 vs beta.10",
			v1:       "0.1.0-beta.2",
			v2:       "0.1.0-beta.10",
			expected: -1,
			desc:     "beta.2 should be less than beta.10",
		},
		{
			name:     "beta.99 vs beta.100",
			v1:       "0.1.0-beta.99",
			v2:       "0.1.0-beta.100",
			expected: -1,
			desc:     "beta.99 should be less than beta.100",
		},

		// Major version comparisons
		{
			name:     "major: 1.0.0 vs 2.0.0",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: -1,
			desc:     "version 1 should be less than version 2",
		},
		{
			name:     "major: 2.0.0 vs 1.9.9",
			v1:       "2.0.0",
			v2:       "1.9.9",
			expected: 1,
			desc:     "version 2 should be greater than 1.9.9",
		},
		{
			name:     "major: 10.0.0 vs 9.99.99",
			v1:       "10.0.0",
			v2:       "9.99.99",
			expected: 1,
			desc:     "version 10 should be greater than 9.99.99",
		},

		// Minor version comparisons
		{
			name:     "minor: 0.1.0 vs 0.2.0",
			v1:       "0.1.0",
			v2:       "0.2.0",
			expected: -1,
			desc:     "0.1.0 should be less than 0.2.0",
		},
		{
			name:     "minor: 1.10.0 vs 1.9.0",
			v1:       "1.10.0",
			v2:       "1.9.0",
			expected: 1,
			desc:     "1.10.0 should be greater than 1.9.0 (not string comparison)",
		},
		{
			name:     "minor: 2.99.0 vs 2.100.0",
			v1:       "2.99.0",
			v2:       "2.100.0",
			expected: -1,
			desc:     "2.99.0 should be less than 2.100.0",
		},

		// Patch version comparisons
		{
			name:     "patch: 1.0.0 vs 1.0.1",
			v1:       "1.0.0",
			v2:       "1.0.1",
			expected: -1,
			desc:     "1.0.0 should be less than 1.0.1",
		},
		{
			name:     "patch: 1.2.10 vs 1.2.9",
			v1:       "1.2.10",
			v2:       "1.2.9",
			expected: 1,
			desc:     "1.2.10 should be greater than 1.2.9",
		},
		{
			name:     "patch: 3.4.99 vs 3.4.100",
			v1:       "3.4.99",
			v2:       "3.4.100",
			expected: -1,
			desc:     "3.4.99 should be less than 3.4.100",
		},

		// Pre-release vs stable comparisons
		{
			name:     "stable vs beta",
			v1:       "0.1.0",
			v2:       "0.1.0-beta.10",
			expected: 1,
			desc:     "stable 0.1.0 should be greater than 0.1.0-beta.10",
		},
		{
			name:     "beta vs stable",
			v1:       "0.1.0-beta.99",
			v2:       "0.1.0",
			expected: -1,
			desc:     "0.1.0-beta.99 should be less than stable 0.1.0",
		},
		{
			name:     "rc vs beta",
			v1:       "1.0.0-rc.1",
			v2:       "1.0.0-beta.99",
			expected: 1,
			desc:     "rc.1 should be greater than beta.99 (alphabetically)",
		},
		{
			name:     "alpha vs beta",
			v1:       "1.0.0-alpha.1",
			v2:       "1.0.0-beta.1",
			expected: -1,
			desc:     "alpha.1 should be less than beta.1",
		},

		// Equal versions
		{
			name:     "equal stable",
			v1:       "1.2.3",
			v2:       "1.2.3",
			expected: 0,
			desc:     "same versions should be equal",
		},
		{
			name:     "equal beta",
			v1:       "0.1.0-beta.5",
			v2:       "0.1.0-beta.5",
			expected: 0,
			desc:     "same beta versions should be equal",
		},

		// Edge cases
		{
			name:     "missing patch version",
			v1:       "1.0",
			v2:       "1.0.0",
			expected: 0,
			desc:     "1.0 should equal 1.0.0",
		},
		{
			name:     "missing minor and patch",
			v1:       "2",
			v2:       "2.0.0",
			expected: 0,
			desc:     "2 should equal 2.0.0",
		},
		{
			name:     "complex pre-release",
			v1:       "1.0.0-beta.1.fix",
			v2:       "1.0.0-beta.1",
			expected: 1,
			desc:     "longer pre-release should be greater",
		},
		// Note: compareVersions doesn't handle 'v' prefix - that's done in isNewerVersion
		// This test shows that raw compareVersions treats them as different strings
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("%s: expected %d, got %d", tt.desc, tt.expected, result)
				t.Errorf("  v1: %s", tt.v1)
				t.Errorf("  v2: %s", tt.v2)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
		desc     string
	}{
		// The critical beta version bug
		{
			name:     "beta.9 to beta.10 update",
			current:  "0.1.0-beta.9",
			latest:   "0.1.0-beta.10",
			expected: true,
			desc:     "should detect beta.10 as newer than beta.9",
		},
		{
			name:     "beta.10 to beta.9 no update",
			current:  "0.1.0-beta.10",
			latest:   "0.1.0-beta.9",
			expected: false,
			desc:     "should not downgrade from beta.10 to beta.9",
		},

		// Dev version handling
		{
			name:     "dev version always updates",
			current:  "dev",
			latest:   "0.0.1",
			expected: true,
			desc:     "dev version should always allow updates",
		},
		{
			name:     "unknown version always updates",
			current:  "unknown",
			latest:   "1.0.0",
			expected: true,
			desc:     "unknown version should always allow updates",
		},

		// Version prefix handling
		{
			name:     "v prefix in current",
			current:  "v1.0.0",
			latest:   "1.0.1",
			expected: true,
			desc:     "should handle v prefix in current version",
		},
		{
			name:     "v prefix in latest",
			current:  "1.0.0",
			latest:   "v1.0.1",
			expected: true,
			desc:     "should handle v prefix in latest version",
		},
		{
			name:     "v prefix in both",
			current:  "v1.0.0",
			latest:   "v1.0.1",
			expected: true,
			desc:     "should handle v prefix in both versions",
		},

		// Normal update scenarios
		{
			name:     "major version update",
			current:  "1.0.0",
			latest:   "2.0.0",
			expected: true,
			desc:     "should update major version",
		},
		{
			name:     "minor version update",
			current:  "1.1.0",
			latest:   "1.2.0",
			expected: true,
			desc:     "should update minor version",
		},
		{
			name:     "patch version update",
			current:  "1.1.1",
			latest:   "1.1.2",
			expected: true,
			desc:     "should update patch version",
		},
		{
			name:     "beta to stable update",
			current:  "1.0.0-beta.99",
			latest:   "1.0.0",
			expected: true,
			desc:     "should update from beta to stable",
		},
		{
			name:     "same version no update",
			current:  "1.0.0",
			latest:   "1.0.0",
			expected: false,
			desc:     "should not update same version",
		},
		{
			name:     "downgrade prevention",
			current:  "2.0.0",
			latest:   "1.9.9",
			expected: false,
			desc:     "should not downgrade",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				currentVersion: tt.current,
			}
			result := s.isNewerVersion(tt.latest)
			if result != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.desc, tt.expected, result)
				t.Errorf("  current: %s", tt.current)
				t.Errorf("  latest:  %s", tt.latest)
			}
		})
	}
}

func TestCompareVersionParts(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal three parts", "1.2.3", "1.2.3", 0},
		{"equal two parts", "1.2", "1.2", 0},
		{"equal one part", "1", "1", 0},
		{"different lengths equal", "1.0.0", "1", 0},
		{"major difference", "2.0.0", "1.0.0", 1},
		{"minor difference", "1.2.0", "1.1.0", 1},
		{"patch difference", "1.1.2", "1.1.1", 1},
		{"double digit major", "10.0.0", "9.0.0", 1},
		{"double digit minor", "1.10.0", "1.9.0", 1},
		{"double digit patch", "1.1.10", "1.1.9", 1},
		{"triple digit handling", "1.0.100", "1.0.99", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersionParts(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("compareVersionParts(%s, %s) = %d; want %d",
					tt.v1, tt.v2, result, tt.expected)
			}
			// Test symmetry
			reverseResult := compareVersionParts(tt.v2, tt.v1)
			if reverseResult != -tt.expected {
				t.Errorf("compareVersionParts(%s, %s) not symmetric: got %d, expected %d",
					tt.v2, tt.v1, reverseResult, -tt.expected)
			}
		})
	}
}

func TestComparePreRelease(t *testing.T) {
	tests := []struct {
		name     string
		pr1      string
		pr2      string
		expected int
	}{
		{"equal beta versions", "beta.1", "beta.1", 0},
		{"beta numeric comparison", "beta.2", "beta.10", -1},
		{"beta double digit", "beta.9", "beta.10", -1},
		{"beta triple digit", "beta.99", "beta.100", -1},
		{"alpha vs beta", "alpha.1", "beta.1", -1},
		{"beta vs rc", "beta.99", "rc.1", -1},
		{"rc numeric", "rc.1", "rc.2", -1},
		{"complex version", "beta.1.fix", "beta.1", 1},
		{"mixed string and number", "beta.1", "beta.1a", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparePreRelease(tt.pr1, tt.pr2)
			if result != tt.expected {
				t.Errorf("comparePreRelease(%s, %s) = %d; want %d",
					tt.pr1, tt.pr2, result, tt.expected)
			}
		})
	}
}

// Benchmark to ensure performance is acceptable
func BenchmarkCompareVersions(b *testing.B) {
	versions := []struct {
		v1, v2 string
	}{
		{"0.1.0-beta.9", "0.1.0-beta.10"},
		{"1.2.3", "1.2.4"},
		{"10.20.30", "10.20.31"},
		{"1.0.0-alpha.1", "1.0.0-beta.1"},
	}

	for _, v := range versions {
		b.Run(v.v1+"_vs_"+v.v2, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = compareVersions(v.v1, v.v2)
			}
		})
	}
}
