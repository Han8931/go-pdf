package app

import "testing"

func TestExtractDOIFromText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "link",
			text: "Find it at https://doi.org/10.1000/ABC.DEF",
			want: "10.1000/abc.def",
		},
		{
			name: "prefix",
			text: "DOI: 10.1234/XYZ-1234.",
			want: "10.1234/xyz-1234",
		},
		{
			name: "bare",
			text: "reference 10.5555/foo.bar-baz",
			want: "10.5555/foo.bar-baz",
		},
		{
			name: "missing",
			text: "no id here",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDOIFromText(tt.text); got != tt.want {
				t.Fatalf("extractDOIFromText(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestExtractArxivIDFromString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "modern",
			text: "arXiv:2101.01234v2",
			want: "2101.01234v2",
		},
		{
			name: "legacy",
			text: "hep-th/9901001",
			want: "hep-th/9901001",
		},
		{
			name: "none",
			text: "no arxiv code",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractArxivIDFromString(tt.text); got != tt.want {
				t.Fatalf("extractArxivIDFromString(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
