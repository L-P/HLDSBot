package hlds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchivePathHasPrefix(t *testing.T) {
	cases := []struct {
		path, prefix string
		expected     bool
	}{
		{"foo", "bar", false},
		{"foo", "", true},
		{"foo", ".", true},
		{"foo", "./", true},
		{"foo/bar", "bar", false},
		{"bar/foo", "bar", true},
		{"bar/foo", "bar/", true},

		{"barf/foo", "bar/", false},
		{"barf/foo", "bar", false},
	}

	for i, v := range cases {
		require.Equal(
			t,
			v.expected,
			archivePathHasPrefix(v.path, v.prefix),
			"case #%d", i,
		)
	}
}

func TestArchivePathTrimPrefix(t *testing.T) {
	cases := []struct {
		path, prefix, expected string
	}{
		{"foo", "bar", "foo"},
		{"foo/bar/baz", "bar", "foo/bar/baz"},
		{"foo/bar/baz", "foo", "bar/baz"},
		{"foo/bar/baz", "foo/", "bar/baz"},
	}

	for i, v := range cases {
		require.Equal(
			t,
			v.expected,
			archivePathTrimPrefix(v.path, v.prefix),
			"case #%d", i,
		)
	}
}
